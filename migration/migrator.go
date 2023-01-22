package migration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sync"

	"github.com/natenho/go-jira"
	"github.com/natenho/go-jira-migrate/internal"
	"github.com/pkg/errors"
)

type Result struct {
	SourceKey     string
	SourceSummary string
	TargetKey     string
	Errors        []error
}

const maxResultsPerSearch = 100

func (r Result) String() string {
	result := "OK"
	if len(r.Errors) > 0 {
		result = fmt.Sprintf("%#v", r.Errors)
	}

	return fmt.Sprintf("%s;%s;%s;%#v", r.SourceKey, r.SourceSummary, r.TargetKey, result)
}

func (r Result) HasTargetIssue() bool {
	return r.TargetKey != ""
}

type Migrator interface {
	Execute(jql string) (chan Result, error)
}

type migrator struct {
	additionalLabels []string
	customFields     []string

	sourceClient *jira.Client
	targetClient *jira.Client

	sourceProjectKey string
	targetProjectKey string

	sourceTargetCustomFieldMap map[string][]jira.Field
	sourceFieldPerIssueType    map[string][]jira.Field
	targetFieldPerIssueType    map[string][]jira.Field

	targetBoard           *jira.Board
	sourceTargetSprintMap map[int]*jira.Sprint

	syncRoot sync.Map

	workerPoolSize int
	importSprints  bool
	deleteOnError  bool
}

type Option func(m *migrator)

func WithAdditionalLabels(labels ...string) Option {
	return func(m *migrator) {
		for _, label := range labels {
			if label != "" {
				m.additionalLabels = append(m.additionalLabels, label)
			}
		}
	}
}

func WithWorkerPoolSize(size int) Option {
	return func(m *migrator) {
		if size <= 0 {
			return
		}

		m.workerPoolSize = size
	}
}

func WithCustomFields(customFieldNames ...string) Option {
	return func(m *migrator) {
		m.customFields = customFieldNames
	}
}

func WithSprints(value bool) Option {
	return func(m *migrator) {
		m.importSprints = value
	}
}

func WithDeleteOnError(value bool) Option {
	return func(m *migrator) {
		m.deleteOnError = value
	}
}

func NewMigrator(sourceUrl, targetUrl, user, apiToken, sourceProjectKey, targetProjectKey string, options ...Option) (Migrator, error) {
	if _, err := url.Parse(sourceUrl); err != nil || sourceUrl == "" {
		return nil, errors.New("Invalid source url")
	}

	if _, err := url.Parse(targetUrl); err != nil || targetUrl == "" {
		return nil, errors.New("Invalid target url")
	}

	if user == "" {
		return nil, errors.New("Invalid user")
	}

	if apiToken == "" {
		return nil, errors.New("Invalid API token")
	}

	transport := jira.BasicAuthTransport{Username: user, Password: apiToken}

	sourceClient, err := jira.NewClient(transport.Client(), sourceUrl)
	if err != nil {
		return nil, err
	}

	targetClient, err := jira.NewClient(transport.Client(), targetUrl)
	if err != nil {
		return nil, err
	}

	m := &migrator{
		sourceClient:               sourceClient,
		targetClient:               targetClient,
		sourceProjectKey:           sourceProjectKey,
		targetProjectKey:           targetProjectKey,
		sourceTargetSprintMap:      map[int]*jira.Sprint{},
		targetFieldPerIssueType:    map[string][]jira.Field{},
		sourceTargetCustomFieldMap: map[string][]jira.Field{},
		syncRoot:                   sync.Map{},
	}

	for _, option := range options {
		option(m)
	}

	return m, nil
}

func (s *migrator) Execute(jql string) (chan Result, error) {
	results := make(chan Result)

	if err := s.discoverFields(); err != nil {
		close(results)
		return results, err
	}

	sourceBoard, err := getBoard(s.sourceClient, s.sourceProjectKey)
	if err != nil {
		close(results)
		return results, err
	}

	targetBoard, err := getBoard(s.targetClient, s.targetProjectKey)
	if err != nil {
		close(results)
		return results, err
	}

	s.targetBoard = targetBoard

	jql = internal.SanitizeJQL(s.sourceProjectKey, jql)

	if err := s.migrateOpenSprints(sourceBoard.ID, targetBoard.ID); err != nil {
		close(results)
		return results, err
	}

	options := &jira.SearchOptions{
		MaxResults: maxResultsPerSearch,
		Fields:     []string{"key"}}

	issues, response, err := s.sourceClient.Issue.Search(jql, options)
	if err != nil {
		close(results)
		return results, err
	}

	if len(issues) == 0 {
		close(results)
		return results, nil
	}

	issueKeys := make(chan string, s.workerPoolSize)
	workers := &sync.WaitGroup{}

	for i := 0; i < len(issues) && i < s.workerPoolSize; i++ {
		workers.Add(1)
		go s.worker(i, issueKeys, results, workers)
	}

	go func() {
		for {
			for _, issue := range issues {
				issueKeys <- issue.Key
			}

			if response.StartAt+response.MaxResults >= response.Total {
				close(issueKeys)
				workers.Wait()
				close(results)
				return
			}

			options.StartAt += response.MaxResults
			issues, response, err = s.sourceClient.Issue.Search(jql, options)
			if err != nil {
				close(issueKeys)
				close(results)
				return
			}
		}
	}()

	return results, nil
}

func getBoard(client *jira.Client, projectKey string) (*jira.Board, error) {
	boards, response, err := client.Board.GetAllBoards(&jira.BoardListOptions{ProjectKeyOrID: projectKey})
	if err != nil {
		return nil, parseResponseError("GetAllBoards", response, err)
	}

	return &boards.Values[0], nil //TODO Support multiple board migration
}

func (s *migrator) worker(id int, issueKeys <-chan string, results chan<- Result, wg *sync.WaitGroup) {
	for issueKey := range issueKeys {
		results <- s.migrateIssue(issueKey)
	}

	wg.Done()
}

func parseResponseError(operation string, response *jira.Response, err error) error {
	var out bytes.Buffer
	if err == nil {
		return nil
	}

	if response == nil {
		return errors.New("Invalid response")
	}

	responseBody, _ := io.ReadAll(response.Body)
	_ = json.Indent(&out, responseBody, "", "  ")
	return errors.Errorf("%s: %s: %s", operation, err, out.String())
}

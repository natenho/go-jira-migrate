package migration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sync"

	"github.com/natenho/go-jira"
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
	return fmt.Sprintf("%s;%s;%s;%#v", r.SourceKey, r.SourceSummary, r.TargetKey, r.Errors)
}

type Migrator interface {
	Execute(projectKey, jql string) (chan Result, error)
}

type migrator struct {
	additionalLabels []string
	customFields     []string

	sourceClient *jira.Client
	targetClient *jira.Client

	projectKey                 string
	sourceTargetCustomFieldMap map[string]jira.Field
	targetBoard                *jira.Board
	sourceTargetSprintMap      map[int]*jira.Sprint

	sourceFields []jira.Field
	targetFields []jira.Field

	workerPoolSize int
}

type Option func(m *migrator)

func WithAdditionalLabel(label string) Option {
	return func(m *migrator) {
		if label == "" {
			return
		}

		m.additionalLabels = append(m.additionalLabels, label)
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

func NewMigrator(sourceUrl, targetUrl, user, apiToken string, options ...Option) (Migrator, error) {
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
		sourceTargetSprintMap:      map[int]*jira.Sprint{},
		sourceTargetCustomFieldMap: map[string]jira.Field{},
	}

	for _, option := range options {
		option(m)
	}

	return m, nil
}

func (s *migrator) Execute(projectKey, jql string) (chan Result, error) {
	s.projectKey = projectKey

	results := make(chan Result)

	if err := s.mapCustomFields(); err != nil {
		close(results)
		return results, err
	}

	sourceBoard, err := getBoard(s.sourceClient, projectKey)
	if err != nil {
		close(results)
		return results, err
	}

	targetBoard, err := getBoard(s.targetClient, projectKey)
	if err != nil {
		close(results)
		return results, err
	}

	s.targetBoard = targetBoard

	if err := s.migrateOpenSprints(sourceBoard.ID, targetBoard.ID); err != nil {
		close(results)
		return results, err
	}

	options := &jira.SearchOptions{
		MaxResults: maxResultsPerSearch,
		Fields:     []string{"key"}}

	issues, response, err := s.sourceClient.Issue.Search(fmt.Sprintf("project = %s AND %s", projectKey, jql), options)
	if err != nil {
		close(results)
		return results, err
	}

	if len(issues) == 0 {
		close(results)
		return results, nil
	}

	issueIDs := make(chan string, s.workerPoolSize)
	workers := &sync.WaitGroup{}

	for i := 0; i < len(issues) && i < s.workerPoolSize; i++ {
		workers.Add(1)
		go s.worker(i, issueIDs, results, workers)
	}

	go func() {
		for {
			for _, issue := range issues {
				issueIDs <- issue.ID
			}

			if response.StartAt+response.MaxResults >= response.Total {
				close(issueIDs)
				workers.Wait()
				close(results)
				return
			}

			options.StartAt += response.MaxResults
			issues, response, err = s.sourceClient.Issue.Search(jql, options)
			if err != nil {
				close(issueIDs)
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

	if len(boards.Values) > 1 {
		return nil, errors.Errorf("Ops, migration of multiple boards is not implemented yet (%d boards found for %s)", len(boards.Values), projectKey)
	}

	return &boards.Values[0], nil
}

func (s *migrator) worker(id int, issueIDs <-chan string, results chan<- Result, wg *sync.WaitGroup) {
	for issueID := range issueIDs {
		results <- s.migrateIssue(issueID)
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

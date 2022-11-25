package migration

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/natenho/go-jira"
	"github.com/pkg/errors"
	"github.com/trivago/tgo/tcontainer"
)

func (s *migrator) migrateIssue(issueKey string) Result {
	result := Result{}

	mutex, _ := s.syncRoot.LoadOrStore(issueKey, &sync.Mutex{})
	mutex.(*sync.Mutex).Lock()
	defer mutex.(*sync.Mutex).Unlock()

	sourceIssue, err := s.getSourceIssueByKey(issueKey)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	if sourceIssue.Fields.Project.Key != s.projectKey {
		return result
	}

	result.SourceKey = sourceIssue.Key
	result.SourceSummary = sourceIssue.Fields.Summary

	existingIssue, err := s.findTargetIssueBySummary(sourceIssue.Fields.Summary)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	if existingIssue != nil {
		result.TargetKey = existingIssue.Key
		result.Errors = append(result.Errors, errors.New("issue already exists"))
		return result
	}

	targetIssue, err := s.buildTargetIssue(sourceIssue)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	if err := s.setupTargetEpic(sourceIssue, targetIssue); err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	createdIssue, response, err := s.targetClient.Issue.Create(targetIssue)
	defer response.Body.Close()

	if err != nil {
		result.Errors = append(result.Errors, parseResponseError("Create", response, errors.Wrapf(err, "could not migrate %s", sourceIssue.Key)))
		return result
	}

	result.TargetKey = createdIssue.Key

	if err := s.setupTargetSprint(sourceIssue, createdIssue); err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	for err := range s.migrateComments(sourceIssue, createdIssue) {
		if err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	for err := range s.migrateAttachments(sourceIssue, createdIssue) {
		if err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	if err := s.linkToOriginalIssue(sourceIssue, createdIssue); err != nil {
		result.Errors = append(result.Errors, err)
	}

	for err := range s.migrateLinks(sourceIssue, createdIssue) {
		if err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	return result
}

func (s *migrator) getSourceIssueByKey(issueKey string) (*jira.Issue, error) {
	issue, response, err := s.sourceClient.Issue.Get(issueKey, nil)
	if err != nil {
		return nil, parseResponseError("Get", response, err)
	}
	defer response.Body.Close()

	return issue, nil
}

func (s *migrator) buildTargetIssue(sourceIssue *jira.Issue) (*jira.Issue, error) {
	targetIssue := &jira.Issue{
		Key: sourceIssue.Key,
		Fields: &jira.IssueFields{
			Type:        jira.IssueType{Name: sourceIssue.Fields.Type.Name},
			Project:     jira.Project{Key: sourceIssue.Fields.Project.Key},
			Description: sourceIssue.Fields.Description,
			Summary:     sourceIssue.Fields.Summary,
			Labels:      sourceIssue.Fields.Labels,
			Unknowns:    tcontainer.NewMarshalMap(),
		},
	}

	if canSetAssignee(sourceIssue) {
		targetIssue.Fields.Assignee = sourceIssue.Fields.Assignee
	}

	if canSetReporter(sourceIssue) {
		targetIssue.Fields.Reporter = sourceIssue.Fields.Reporter
	}

	url := s.getSourceUrl(sourceIssue)
	created := time.Time(sourceIssue.Fields.Created)
	targetIssue.Fields.Description = fmt.Sprintf(
		"%s\n\n{color:red}_Original issue [%s|%s] created on %s by *%s*_{color}",
		targetIssue.Fields.Description,
		sourceIssue.Key,
		url,
		created,
		sourceIssue.Fields.Reporter.DisplayName)

	targetIssue.Fields.Labels = append(targetIssue.Fields.Labels, s.additionalLabels...)

	if canSetPriority(sourceIssue) {
		targetIssue.Fields.Priority = &jira.Priority{Name: sourceIssue.Fields.Priority.Name}
	}

	if canSetCustomFields(sourceIssue) {
		for srcCustomFieldID, srcCustomFieldValue := range sourceIssue.Fields.Unknowns {
			if targetField, ok := s.sourceTargetCustomFieldMap[srcCustomFieldID]; ok {
				targetIssue.Fields.Unknowns[targetField.Key] = srcCustomFieldValue
			}
		}
	}

	return targetIssue, nil
}

func canSetAssignee(sourceIssue *jira.Issue) bool {
	return sourceIssue.Fields.Assignee != nil && sourceIssue.Fields.Assignee.Active
}

func canSetReporter(sourceIssue *jira.Issue) bool {
	return sourceIssue.Fields.Reporter != nil && sourceIssue.Fields.Reporter.Active
}

func canSetPriority(sourceIssue *jira.Issue) bool {
	return sourceIssue.Fields.Type.Name != "Epic" &&
		sourceIssue.Fields.Type.Name != "Subtask" &&
		sourceIssue.Fields.Priority != nil
}

func canSetCustomFields(sourceIssue *jira.Issue) bool {
	return sourceIssue.Fields.Type.Name != "Epic"
}

func (s *migrator) getSourceUrl(sourceIssue *jira.Issue) string {
	sourceBaseUrl := s.sourceClient.GetBaseURL()
	url, _ := url.JoinPath(sourceBaseUrl.String(), "/browse", sourceIssue.Key)
	return url
}

func (s *migrator) findTargetIssueBySummary(summary string) (*jira.Issue, error) {
	searchTerm := sanitizeForJQL(summary)

	existingIssue, response, err := s.targetClient.Issue.
		Search(fmt.Sprintf(`project = %s AND summary ~ "%s"`, s.projectKey, searchTerm),
			&jira.SearchOptions{MaxResults: maxResultsPerSearch, Fields: []string{"key", "summary", "description"}})
	if err != nil {
		return nil, parseResponseError("issueExists", response, err)
	}

	defer response.Body.Close()

	if len(existingIssue) > 0 && strings.EqualFold(existingIssue[0].Fields.Summary, summary) {
		return &existingIssue[0], nil
	}

	return nil, nil
}

func sanitizeForJQL(input string) string {
	input = strings.ReplaceAll(input, " - ", " ")
	input = strings.ReplaceAll(input, "- ", " ")
	input = strings.ReplaceAll(input, " -", " ")

	const notAllowedSearchChars string = `["](/\)?`

	filter := func(r rune) rune {
		if strings.ContainsRune(notAllowedSearchChars, r) {
			return ' '
		}
		return r
	}

	return strings.Map(filter, input)
}

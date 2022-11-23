package migration

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/natenho/go-jira"
	"github.com/natenho/go-jira-migrate/internal"
	"github.com/pkg/errors"
	"github.com/trivago/tgo/tcontainer"
)

func (s *migrator) migrateIssue(issueID string) Result {
	result := Result{}

	sourceIssue, err := s.getSourceIssueByID(issueID)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	if sourceIssue.Fields.Project.Key != s.projectKey {
		result.Errors = append(result.Errors, errors.Errorf("unable migrate: %s does not belong to %s", sourceIssue.Key, s.projectKey))
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

func (s *migrator) getSourceIssueByID(issueID string) (*jira.Issue, error) {
	issue, response, err := s.sourceClient.Issue.Get(issueID, nil)
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

	if sourceIssue.Fields.Assignee != nil && sourceIssue.Fields.Assignee.Active {
		targetIssue.Fields.Assignee = sourceIssue.Fields.Assignee
	}

	if sourceIssue.Fields.Reporter != nil && sourceIssue.Fields.Reporter.Active {
		targetIssue.Fields.Reporter = sourceIssue.Fields.Reporter
	}

	url := s.getSourceUrl(sourceIssue)
	created := time.Time(sourceIssue.Fields.Created)
	targetIssue.Fields.Description = fmt.Sprintf(
		"%s\n\n{color:red}_Original issue %s created on %s by %s_{color}",
		targetIssue.Fields.Description,
		url,
		created,
		sourceIssue.Fields.Reporter.DisplayName)

	targetIssue.Fields.Labels = append(targetIssue.Fields.Labels, s.additionalLabels...)

	if sourceIssue.Fields.Type.Name != "Epic" { //TODO Dark magic to migrate Epic with no errors
		targetIssue.Fields.Priority = &jira.Priority{Name: sourceIssue.Fields.Priority.Name}

		for srcCustomFieldID, srcCustomFieldValue := range sourceIssue.Fields.Unknowns {
			if targetField, ok := s.sourceTargetCustomFieldMap[srcCustomFieldID]; ok {
				targetIssue.Fields.Unknowns[targetField.Key] = srcCustomFieldValue
			}
		}
	}

	return targetIssue, nil
}

func (s *migrator) getSourceUrl(sourceIssue *jira.Issue) string {
	sourceBaseUrl := s.sourceClient.GetBaseURL()
	url, _ := url.JoinPath(sourceBaseUrl.String(), "/browse", sourceIssue.Key)
	return url
}

func (s *migrator) findTargetIssueBySummary(summary string) (*jira.Issue, error) {
	const notAllowedSearchChars string = `["](/\)?`

	searchTerm := internal.RemoveCharacters(summary, notAllowedSearchChars)

	existingIssue, response, err := s.targetClient.Issue.
		Search(fmt.Sprintf(`project = %s AND summary ~ "%s"`, s.projectKey, searchTerm),
			&jira.SearchOptions{MaxResults: maxResultsPerSearch, Fields: []string{"key", "summary"}})
	if err != nil {
		return nil, parseResponseError("issueExists", response, err)
	}

	defer response.Body.Close()

	if len(existingIssue) > 0 && strings.EqualFold(existingIssue[0].Fields.Summary, summary) {
		return &existingIssue[0], nil
	}

	return nil, nil
}

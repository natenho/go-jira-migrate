package migration

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/natenho/go-jira"
	"github.com/natenho/go-jira-migrate/internal"
	"github.com/pkg/errors"
	"github.com/trivago/tgo/tcontainer"
)

func (s *migrator) migrateIssue(issueID string) Result {
	result := Result{}

	sourceIssue, response, err := s.sourceClient.Issue.Get(issueID, nil)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}
	defer response.Body.Close()

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

	targetIssue, response, err = s.targetClient.Issue.Create(targetIssue)
	defer response.Body.Close()

	if err != nil {
		result.Errors = append(result.Errors, parseResponseError("Create", response, err))
		return result
	}

	if err := s.setupTargetSprint(sourceIssue, targetIssue); err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	for err := range s.migrateComments(sourceIssue, targetIssue) {
		result.Errors = append(result.Errors, err)
	}

	for err := range s.migrateAttachments(sourceIssue, targetIssue) {
		result.Errors = append(result.Errors, err)
	}

	//TODO
	// for _, item := range sourceIssue.Fields.IssueLinks {
	// 	log.Printf("%#v ", item)
	// }

	log.Printf(";OK")

	return Result{SourceKey: sourceIssue.Key, TargetKey: targetIssue.Key}
}

func (s *migrator) buildTargetIssue(sourceIssue *jira.Issue) (*jira.Issue, error) {
	targetIssue := &jira.Issue{
		Key: sourceIssue.Key,
		Fields: &jira.IssueFields{
			Type: jira.IssueType{Name: sourceIssue.Fields.Type.Name},
			//TODO Priority:    sourceIssue.Fields.Priority,
			Project:     jira.Project{Key: sourceIssue.Fields.Project.Key},
			Assignee:    sourceIssue.Fields.Assignee,
			Description: sourceIssue.Fields.Description,
			Summary:     sourceIssue.Fields.Summary,
			Reporter:    sourceIssue.Fields.Reporter,
			Labels:      sourceIssue.Fields.Labels,
			Unknowns:    tcontainer.NewMarshalMap(),
		},
	}

	sourceBaseUrl := s.sourceClient.GetBaseURL()
	sourceIssueUrl, _ := url.JoinPath(sourceBaseUrl.String(), "/browse", sourceIssue.Key)
	targetIssue.Fields.Description = fmt.Sprintf("{panel:bgColor=#fffae6}\nOriginal issue: %s\n{panel}\n%s", sourceIssueUrl, targetIssue.Fields.Description)

	targetIssue.Fields.Labels = append(targetIssue.Fields.Labels, s.additionalLabels...)

	for srcCustomFieldID, srcCustomFieldValue := range sourceIssue.Fields.Unknowns {
		if targetField, ok := s.sourceTargetCustomFieldMap[srcCustomFieldID]; ok {
			targetIssue.Fields.Unknowns[targetField.Key] = srcCustomFieldValue
		}
	}

	return targetIssue, nil
}

func (s *migrator) findTargetIssueBySummary(summary string) (*jira.Issue, error) {
	const notAllowedSearchChars string = "[]"

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

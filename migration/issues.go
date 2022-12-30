package migration

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/natenho/go-jira"
	"github.com/natenho/go-jira-migrate/internal"
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

	if sourceIssue.Fields.Project.Key != s.sourceProjectKey {
		result.Errors = append(result.Errors, errors.Errorf("issue %s does not belong to %s", sourceIssue.Key, s.sourceProjectKey))
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

	if err := s.migrateParent(sourceIssue, targetIssue); err != nil {
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
			Project:     jira.Project{Key: s.targetProjectKey},
			Description: sourceIssue.Fields.Description,
			Summary:     sourceIssue.Fields.Summary,
			Labels:      sourceIssue.Fields.Labels,
			Unknowns:    tcontainer.NewMarshalMap(),
		},
	}

	if s.canSetAssignee(sourceIssue) {
		targetIssue.Fields.Assignee = sourceIssue.Fields.Assignee
	}

	if s.canSetReporter(sourceIssue) {
		targetIssue.Fields.Reporter = sourceIssue.Fields.Reporter
	}

	url := s.getSourceUrl(sourceIssue)
	created := time.Time(sourceIssue.Fields.Created)
	targetIssue.Fields.Description = fmt.Sprintf(
		"%s\n\n{color:red}_Original issue [%s|%s] created on %s by [~accountid:%s]_{color}",
		targetIssue.Fields.Description,
		sourceIssue.Key,
		url,
		created,
		sourceIssue.Fields.Reporter.AccountID)

	targetIssue.Fields.Labels = append(targetIssue.Fields.Labels, s.additionalLabels...)

	if s.canMigrateField(sourceIssue.Fields.Type.Name, "priority") {
		targetIssue.Fields.Priority = &jira.Priority{Name: sourceIssue.Fields.Priority.Name}
	}

	for _, targetField := range s.targetFieldPerIssueType[targetIssue.Fields.Type.Name] {
		sourceFieldKeys := s.getSourceFieldsFromTargetFieldKey(targetField.Key)

		if len(sourceFieldKeys) == 0 {
			continue
		}

		for _, sourceFieldKey := range sourceFieldKeys {

			fieldValue, ok := sourceIssue.Fields.Unknowns[sourceFieldKey].(map[string]interface{})
			if ok {
				delete(fieldValue, "id")
				delete(fieldValue, "self")
				targetIssue.Fields.Unknowns[targetField.Key] = fieldValue
				continue
			}

			if targetField.Name == "Flagged" && sourceIssue.Fields.Unknowns[sourceFieldKey] != nil { //TODO Get rid of this dark magic
				targetIssue.Fields.Unknowns[targetField.Key] = []interface{}{map[string]interface{}{"value": "Impediment"}}
				continue
			}

			targetIssue.Fields.Unknowns[targetField.Key] = sourceIssue.Fields.Unknowns[sourceFieldKey]
		}
	}

	return targetIssue, nil
}

func (s *migrator) canSetAssignee(sourceIssue *jira.Issue) bool {
	if sourceIssue.Fields.Assignee == nil {
		return false
	}
	user, _, _ := s.targetClient.User.GetByAccountID(sourceIssue.Fields.Assignee.AccountID) //TODO Could be cached for optimization
	return user != nil && user.Active
}

func (s *migrator) canSetReporter(sourceIssue *jira.Issue) bool {
	if sourceIssue.Fields.Reporter == nil {
		return false
	}
	user, _, _ := s.targetClient.User.GetByAccountID(sourceIssue.Fields.Reporter.AccountID) //TODO Could be cached for optimization
	return user != nil && user.Active
}

func (s *migrator) getSourceUrl(sourceIssue *jira.Issue) string {
	sourceBaseUrl := s.sourceClient.GetBaseURL()
	url, _ := url.JoinPath(sourceBaseUrl.String(), "/browse", sourceIssue.Key)
	return url
}

func (s *migrator) findTargetIssueBySummary(summary string) (*jira.Issue, error) {
	searchTerm := internal.SanitizeTermForJQL(summary)

	searchResult, response, err := s.targetClient.Issue.
		Search(fmt.Sprintf(`project = %s AND summary ~ "%s"`, s.targetProjectKey, searchTerm),
			&jira.SearchOptions{MaxResults: maxResultsPerSearch, Fields: []string{"key", "summary"}})
	if err != nil {
		return nil, parseResponseError("issueExists", response, err)
	}

	defer response.Body.Close()

	for _, existingIssue := range searchResult {
		if strings.EqualFold(existingIssue.Fields.Summary, summary) {
			return &existingIssue, nil
		}
	}

	return nil, nil
}

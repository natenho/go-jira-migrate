package migration

import "github.com/natenho/go-jira"

func (s *migrator) migrateParent(sourceIssue *jira.Issue, targetIssue *jira.Issue) error {
	if sourceIssue.Fields.Parent == nil {
		return nil
	}

	parentIssue, response, err := s.sourceClient.Issue.Get(sourceIssue.Fields.Parent.ID, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if parentIssue.Fields.Project.Key != s.sourceProjectKey {
		return nil
	}

	parentIssueMigrateResult := s.migrateIssue(parentIssue.Key)
	if !parentIssueMigrateResult.HasTargetIssue() {
		return parentIssueMigrateResult.Errors[0]
	}

	targetIssue.Fields.Parent = &jira.Parent{Key: parentIssueMigrateResult.TargetKey}

	return nil
}

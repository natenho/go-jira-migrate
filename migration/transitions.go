package migration

import (
	"strings"

	"github.com/natenho/go-jira"
)

func (s *migrator) migrateStatus(sourceIssue *jira.Issue, targetIssue *jira.Issue) chan error {
	errChan := make(chan error, len(sourceIssue.Fields.Attachments))

	targetTransitions, response, err := s.targetClient.Issue.GetTransitions(targetIssue.Key)
	if err != nil {
		errChan <- parseResponseError("GetTransitions", response, err)
	}

	for _, targetTransition := range targetTransitions {
		if strings.EqualFold(targetTransition.To.StatusCategory.Name, sourceIssue.Fields.Status.StatusCategory.Name) {
			if response, err := s.targetClient.Issue.DoTransition(targetIssue.Key, targetTransition.ID); err != nil {
				errChan <- parseResponseError("DoTransition", response, err)
			}

			break
		}
	}

	close(errChan)
	return errChan
}

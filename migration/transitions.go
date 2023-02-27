package migration

import (
	"strings"
	"sync"

	"github.com/natenho/go-jira"
)

func (s *migrator) migrateStatus(sourceIssue *jira.Issue, targetIssue *jira.Issue) chan error {
	wg := &sync.WaitGroup{}

	errChan := make(chan error, 1)
	defer close(errChan)

	targetTransitions, response, err := s.targetClient.Issue.GetTransitions(targetIssue.Key)
	if err != nil {
		go func() {
			errChan <- parseResponseError("GetTransitions", response, err)
		}()
		return errChan
	}

	for _, targetTransition := range targetTransitions {
		if strings.EqualFold(targetTransition.To.StatusCategory.Name, sourceIssue.Fields.Status.StatusCategory.Name) {
			if response, err := s.targetClient.Issue.DoTransition(targetIssue.Key, targetTransition.ID); err != nil {
				wg.Add(1)
				go func() {
					errChan <- parseResponseError("DoTransition", response, err)
					wg.Done()
				}()
			}
			break
		}
	}

	wg.Wait()
	return errChan
}

package migration

import (
	"fmt"
	"sync"

	"github.com/natenho/go-jira"
)

func (s *migrator) migrateComments(sourceIssue *jira.Issue, targetIssue *jira.Issue) chan error {
	wg := &sync.WaitGroup{}

	errChan := make(chan error, len(sourceIssue.Fields.Attachments))
	defer close(errChan)

	for _, item := range sourceIssue.Fields.Comments.Comments {
		item.Body = fmt.Sprintf("_On %s [~accountid:%s] wrote:_\n\n%s", item.Created, item.Author.AccountID, item.Body)
		_, response, err := s.targetClient.Issue.AddComment(targetIssue.ID, item)
		if err != nil {
			wg.Add(1)
			go func() {
				errChan <- parseResponseError("AddComment", response, err)
				wg.Done()
			}()
		}
	}

	wg.Wait()
	return errChan
}

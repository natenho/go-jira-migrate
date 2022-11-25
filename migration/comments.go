package migration

import (
	"fmt"

	"github.com/natenho/go-jira"
)

func (s *migrator) migrateComments(sourceIssue *jira.Issue, targetIssue *jira.Issue) chan error {
	errChan := make(chan error, len(sourceIssue.Fields.Attachments))

	for _, item := range sourceIssue.Fields.Comments.Comments {
		item.Body = fmt.Sprintf("_On %s %s <%s> wrote:_\n\n%s", item.Created, item.Author.DisplayName, item.Author.EmailAddress, item.Body)
		_, response, err := s.targetClient.Issue.AddComment(targetIssue.ID, item)
		if err != nil {
			errChan <- parseResponseError("AddComment", response, err)
		}
	}
	close(errChan)
	return errChan
}

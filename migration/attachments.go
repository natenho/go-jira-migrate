package migration

import (
	"sync"

	"github.com/natenho/go-jira"
	"github.com/pkg/errors"
)

func (s *migrator) migrateAttachments(sourceIssue *jira.Issue, targetIssue *jira.Issue) chan error {
	wg := &sync.WaitGroup{}

	errChan := make(chan error, len(sourceIssue.Fields.Attachments))
	defer close(errChan)

	for _, item := range sourceIssue.Fields.Attachments {
		wg.Add(1)
		go func(item *jira.Attachment) {
			errChan <- s.migrateAttachment(item, targetIssue.ID)
			wg.Done()
		}(item)
	}

	wg.Wait()
	return errChan
}

func (s *migrator) migrateAttachment(attachment *jira.Attachment, targetIssueID string) (err error) {
	response, err := s.sourceClient.Issue.DownloadAttachment(attachment.ID)
	if err != nil {
		return errors.Errorf("Could not download attachment %s %s", attachment.Filename, err)
	}
	defer response.Body.Close()

	_, response, err = s.targetClient.Issue.PostAttachment(targetIssueID, response.Body, attachment.Filename)

	return parseResponseError("migrateAttachment", response, err)
}

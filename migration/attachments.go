package migration

import (
	"fmt"
	"net/http"
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
	if attachment == nil {
		return errors.New("Invalid attachment")
	}

	response, err := s.sourceClient.Issue.DownloadAttachment(attachment.ID)
	if err != nil {
		return errors.Errorf("Could not download attachment %s %s", attachment.Filename, err)
	}

	var attachmentSize int64
	if response != nil {
		defer response.Body.Close()
		attachmentSize = response.ContentLength
	}

	_, postResponse, err := s.targetClient.Issue.PostAttachment(targetIssueID, response.Body, attachment.Filename)
	if postResponse != nil {
		defer postResponse.Body.Close()
	}

	if postResponse.StatusCode == http.StatusRequestEntityTooLarge {
		err = errors.Wrapf(err, "Review upload limits for target account: Refer to https://support.atlassian.com/jira-cloud-administration/docs/configure-file-attachments/")
	}

	return parseResponseError(fmt.Sprintf("migrateAttachment(%s, %d bytes)", attachment.Filename, attachmentSize), postResponse, err)
}

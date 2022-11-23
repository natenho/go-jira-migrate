package migration

import (
	"fmt"
	"sync"

	"github.com/natenho/go-jira"
	"github.com/pkg/errors"
)

func (s *migrator) linkToOriginalIssue(sourceIssue, targetIssue *jira.Issue) error {
	url := s.getSourceUrl(sourceIssue)
	_, response, err := s.targetClient.Issue.AddRemoteLink(targetIssue.ID,
		&jira.RemoteLink{Object: &jira.RemoteLinkObject{
			URL:   url,
			Title: fmt.Sprintf("Original Issue - %s", url),
		}})
	if err != nil {
		return parseResponseError("linkToOriginalIssue", response, err)
	}

	return nil
}

func (s *migrator) migrateLinks(sourceIssue, targetIssue *jira.Issue) chan error {
	wg := &sync.WaitGroup{}

	errChan := make(chan error, len(sourceIssue.Fields.IssueLinks))
	defer close(errChan)

	for _, item := range sourceIssue.Fields.IssueLinks {
		wg.Add(1)
		go func(item *jira.IssueLink) {
			errChan <- s.migrateLink(item, sourceIssue, targetIssue)
			wg.Done()
		}(item)
	}

	wg.Wait()
	return errChan
}

func (s *migrator) migrateLink(link *jira.IssueLink, sourceIssue *jira.Issue, targetIssue *jira.Issue) error {
	var targetInwardIssue, targetOutwardIssue *jira.Issue

	if link.InwardIssue == nil {
		targetInwardIssue = targetIssue
	} else {
		sourceInwardIssue, err := s.getSourceIssueByID(link.InwardIssue.ID)
		if err != nil {
			return err
		}

		targetInwardIssue, err = s.findTargetIssueBySummary(sourceInwardIssue.Fields.Summary)
		if err != nil {
			return err
		}

		if targetInwardIssue == nil && sourceInwardIssue.Fields.Resolution == nil {
			result := s.migrateIssue(sourceInwardIssue.ID)
			if !result.HasTargetIssue() {
				return errors.Errorf("could not create link: %s could not be created on target: %#v", sourceInwardIssue.Key, result.Errors)
			}

			targetInwardIssue = &jira.Issue{Key: result.TargetKey}
		}

		if err := s.remoteLinkToRelatedIssue(sourceInwardIssue, targetIssue, link); err != nil {
			return errors.Errorf("could not remote link to %s: %#v", sourceInwardIssue.Key, err)
		}
	}

	if link.OutwardIssue == nil {
		targetOutwardIssue = targetIssue
	} else {
		sourceOutwardIssue, err := s.getSourceIssueByID(link.OutwardIssue.ID)
		if err != nil {
			return err
		}

		if targetOutwardIssue == nil && sourceOutwardIssue.Fields.Resolution == nil {
			targetOutwardIssue, err = s.findTargetIssueBySummary(sourceOutwardIssue.Fields.Summary)
			if err != nil {
				return err
			}

			if targetOutwardIssue == nil {
				result := s.migrateIssue(sourceOutwardIssue.ID)
				if !result.HasTargetIssue() {
					return errors.Errorf("could not create link: %s could not be created on target: %#v", sourceOutwardIssue.Key, result.Errors)
				}
				targetOutwardIssue = &jira.Issue{Key: result.TargetKey}
			}
		}

		if err := s.remoteLinkToRelatedIssue(sourceOutwardIssue, targetIssue, link); err != nil {
			return errors.Errorf("could not remote link to %s: %#v", sourceOutwardIssue.Key, err)
		}
	}

	if targetInwardIssue == nil || targetOutwardIssue == nil {
		return nil
	}

	response, err := s.targetClient.Issue.AddLink(
		&jira.IssueLink{
			Type:         jira.IssueLinkType{Name: link.Type.Name},
			InwardIssue:  targetInwardIssue,
			OutwardIssue: targetOutwardIssue,
		})
	if err != nil {
		return parseResponseError("AddLink", response, err)
	}

	return nil
}

func (s *migrator) remoteLinkToRelatedIssue(relatedIssue *jira.Issue, targetIssue *jira.Issue, link *jira.IssueLink) error {
	url := s.getSourceUrl(relatedIssue)

	var linkType string

	if link.InwardIssue == nil {
		return nil
	}

	if link.InwardIssue.Key == targetIssue.Key {
		linkType = link.Type.Outward
	} else {
		linkType = link.Type.Inward
	}

	_, response, err := s.targetClient.Issue.AddRemoteLink(targetIssue.ID,
		&jira.RemoteLink{
			Object: &jira.RemoteLinkObject{
				URL:   url,
				Title: fmt.Sprintf("%s %s", linkType, url),
			}})
	if err != nil {
		return parseResponseError("AddRemoteLink", response, err)
	}
	return nil
}

package migration

import (
	"fmt"
	"sync"

	"github.com/natenho/go-jira"
	"github.com/pkg/errors"
)

func (s *migrator) linkToOriginalIssue(sourceIssue, targetIssue *jira.Issue) error {
	return s.linkRemoteIssue(sourceIssue, targetIssue, "Original Issue - ")
}

func (s *migrator) linkRemoteIssue(remoteIssue, targetIssue *jira.Issue, prefix string) error {
	url := s.getSourceUrl(remoteIssue)
	_, response, err := s.targetClient.Issue.AddRemoteLink(targetIssue.Key,
		&jira.RemoteLink{Object: &jira.RemoteLinkObject{
			URL:   url,
			Title: fmt.Sprintf("%s%s", prefix, url),
		}})
	if err != nil {
		return parseResponseError("linkRemoteIssue", response, err)
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
			errChan <- s.migrateLink(item, targetIssue)
			wg.Done()
		}(item)
	}

	wg.Wait()
	return errChan
}

func (s *migrator) migrateLink(link *jira.IssueLink, targetIssue *jira.Issue) error {
	var targetInwardIssue, targetOutwardIssue *jira.Issue

	if link.InwardIssue == nil {
		targetInwardIssue = targetIssue
	} else {
		sourceInwardIssue, err := s.getSourceIssueByKey(link.InwardIssue.Key)
		if err != nil {
			return err
		}

		if err := s.remoteLinkToRelatedIssue(sourceInwardIssue, targetIssue, link); err != nil {
			return errors.Errorf("could not remote link to %s: %#v", sourceInwardIssue.Key, err)
		}

		targetInwardIssue, err = s.findTargetIssueBySummary(sourceInwardIssue.Fields.Summary)
		if err != nil {
			return err
		}

		if targetInwardIssue == nil && s.canMigrateLinkedIssue(sourceInwardIssue) {
			result := s.migrateIssue(sourceInwardIssue.Key)
			if !result.HasTargetIssue() {
				return errors.Errorf("could not create link: %s could not be created on target: %#v", sourceInwardIssue.Key, result.Errors)
			}

			targetInwardIssue = &jira.Issue{Key: result.TargetKey}
		}
	}

	if link.OutwardIssue == nil {
		targetOutwardIssue = targetIssue
	} else {
		sourceOutwardIssue, err := s.getSourceIssueByKey(link.OutwardIssue.Key)
		if err != nil {
			return err
		}

		if err := s.remoteLinkToRelatedIssue(sourceOutwardIssue, targetIssue, link); err != nil {
			return errors.Errorf("could not remote link to %s: %#v", sourceOutwardIssue.Key, err)
		}

		if targetOutwardIssue == nil && s.canMigrateLinkedIssue(sourceOutwardIssue) {
			targetOutwardIssue, err = s.findTargetIssueBySummary(sourceOutwardIssue.Fields.Summary)
			if err != nil {
				return err
			}

			if targetOutwardIssue == nil {
				result := s.migrateIssue(sourceOutwardIssue.Key)
				if !result.HasTargetIssue() {
					return errors.Errorf("could not create link: %s could not be created on target: %#v", sourceOutwardIssue.Key, result.Errors)
				}
				targetOutwardIssue = &jira.Issue{Key: result.TargetKey}
			}
		}
	}

	if targetInwardIssue == nil || targetOutwardIssue == nil {
		return nil
	}

	if targetInwardIssue.Key == targetOutwardIssue.Key {
		return nil
	}

	targetLink := &jira.IssueLink{
		Type:         jira.IssueLinkType{Name: link.Type.Name},
		InwardIssue:  &jira.Issue{Key: targetInwardIssue.Key},
		OutwardIssue: &jira.Issue{Key: targetOutwardIssue.Key},
	}
	response, err := s.targetClient.Issue.AddLink(targetLink)
	if err != nil {
		return parseResponseError("AddLink", response, err)
	}

	return nil
}

func (s *migrator) canMigrateLinkedIssue(linkedIssue *jira.Issue) bool {
	return linkedIssue.Fields.Resolution == nil &&
		linkedIssue.Fields.Project.Key == s.sourceProjectKey
}

func (s *migrator) remoteLinkToRelatedIssue(relatedIssue *jira.Issue, targetIssue *jira.Issue, link *jira.IssueLink) error {
	url := s.getSourceUrl(relatedIssue)

	var linkType string

	if link.InwardIssue == nil && link.OutwardIssue == nil {
		return nil
	}

	if link.InwardIssue == nil || link.InwardIssue.Key == targetIssue.Key {
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

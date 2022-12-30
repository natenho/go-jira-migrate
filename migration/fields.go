package migration

import (
	"strings"

	"github.com/natenho/go-jira"
	"github.com/natenho/go-jira-migrate/internal"
	"golang.org/x/exp/slices"
)

func (s *migrator) canMigrateField(issueType, issueFieldKey string) bool {
	availableFields, ok := s.targetFieldPerIssueType[issueType]
	if !ok {
		return false
	}
	for _, availableField := range availableFields {
		if availableField.Key == issueFieldKey {
			return true
		}
	}

	return false
}

func (s *migrator) discoverFields() error {
	err := s.mapCustomFields()
	if err != nil {
		return err
	}

	s.sourceFieldPerIssueType, err = getAvailableFieldsPerIssueType(s.sourceClient, s.sourceProjectKey)
	if err != nil {
		return err
	}

	s.targetFieldPerIssueType, err = getAvailableFieldsPerIssueType(s.targetClient, s.targetProjectKey)
	if err != nil {
		return err
	}

	return nil
}

func getAvailableFieldsPerIssueType(client *jira.Client, projectKey string) (map[string][]jira.Field, error) {
	availableFieldsMap := map[string][]jira.Field{}

	meta, response, err := client.Issue.GetCreateMeta(projectKey)
	if err != nil {
		return availableFieldsMap, parseResponseError("GetCreateMeta", response, err)
	}

	projectMeta := meta.GetProjectWithKey(projectKey)

	for _, issueType := range projectMeta.IssueTypes {
		for fieldKey := range issueType.Fields {

			fieldName, _ := issueType.Fields.String(fieldKey + "/name")
			customSchema, _ := issueType.Fields.String(fieldKey + "/schema/custom")

			field := jira.Field{
				Key:    fieldKey,
				Name:   fieldName,
				Custom: customSchema != "",
				Schema: jira.FieldSchema{Custom: customSchema}}

			availableFieldsMap[issueType.Name] = append(availableFieldsMap[issueType.Name], field)
		}
	}

	return availableFieldsMap, err
}

func (s *migrator) mapCustomFields() error {
	sourceFields, response, err := s.sourceClient.Field.GetList()
	if err != nil {
		return parseResponseError("GetList", response, err)
	}

	targetFields, response, err := s.targetClient.Field.GetList()
	if err != nil {
		return parseResponseError("GetList", response, err)
	}

	for _, sourceField := range sourceFields {
		if !slices.Contains(s.customFields, sourceField.Name) {
			continue
		}

		for _, targetField := range targetFields {
			if areEquivalentFields(sourceField, targetField) || s.areMappedFields(sourceField, targetField) {
				s.sourceTargetCustomFieldMap[sourceField.Key] = append(s.sourceTargetCustomFieldMap[sourceField.Key], targetField)
			}
		}
	}

	return nil
}

func areEquivalentFields(a, b jira.Field) bool {
	return strings.EqualFold(b.Name, a.Name) && b.Schema.Custom == a.Schema.Custom
}

// TODO Improve this via configuration
func (s *migrator) areMappedFields(sourceField, targetField jira.Field) bool {
	if sourceField.Name == "Story Points" && targetField.Name == "Story point estimate" {
		return true
	}

	return false
}

func (s *migrator) getSourceFieldsFromTargetFieldKey(targetFieldKey string) []string {
	var sourceFieldKeys []string
	for sourceFieldKey, targetFields := range s.sourceTargetCustomFieldMap {
		for _, targetField := range targetFields {
			if targetField.Key == targetFieldKey {
				sourceFieldKeys = append(sourceFieldKeys, sourceFieldKey)
			}
		}
	}

	return sourceFieldKeys
}

func (s *migrator) getCustomFieldValue(issue *jira.Issue, fieldName string) any {
	field, ok := internal.SliceFind(s.sourceFieldPerIssueType[issue.Fields.Type.Name], func(field jira.Field) bool {
		return field.Name == fieldName
	})

	if ok {
		return issue.Fields.Unknowns[field.Key]
	}

	return nil
}

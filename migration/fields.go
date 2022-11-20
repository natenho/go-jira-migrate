package migration

import (
	"github.com/natenho/go-jira"
	"github.com/natenho/go-jira-migrate/internal"
	"golang.org/x/exp/slices"
)

func (s *migrator) mapCustomFields() error {
	sourceFields, response, err := s.sourceClient.Field.GetList()
	if err != nil {
		return parseResponseError("GetList", response, err)
	}

	s.sourceFields = sourceFields

	targetFields, response, err := s.targetClient.Field.GetList()
	if err != nil {
		return parseResponseError("GetList", response, err)
	}

	s.targetFields = targetFields

	for _, sourceField := range sourceFields {
		if !slices.Contains(s.customFields, sourceField.Name) {
			continue
		}

		if targetField, ok := internal.SliceFind(targetFields,
			func(targetField jira.Field) bool {
				return targetField.Name == sourceField.Name &&
					targetField.Schema.Custom == sourceField.Schema.Custom
			}); ok {
			s.sourceTargetCustomFieldMap[sourceField.Key] = targetField
		}
	}

	return nil
}

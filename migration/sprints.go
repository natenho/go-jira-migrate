package migration

import (
	"log"

	"github.com/natenho/go-jira"
	"github.com/natenho/go-jira-migrate/internal"
	"github.com/pkg/errors"
)

func getOpenSprints(client *jira.Client, boardID int) ([]jira.Sprint, error) {
	options := &jira.SearchOptions{
		MaxResults: maxResultsPerSearch,
		Fields:     []string{"key"}}

	var sprints []jira.Sprint

	for {
		pageSprints, response, err := client.Board.GetAllSprintsWithOptions(boardID, &jira.GetAllSprintsOptions{SearchOptions: *options})
		if err != nil {
			return nil, parseResponseError("GetAllSprints", response, err)
		}

		for _, sprint := range pageSprints.Values {
			if sprint.CompleteDate == nil {
				sprints = append(sprints, sprint)
			}
		}

		options.StartAt += pageSprints.MaxResults

		if pageSprints.IsLast {
			break
		}
	}

	return sprints, nil
}

func (s *migrator) migrateOpenSprints(sourceBoardID, targetBoardID int) error {
	if !s.importSprints {
		return nil
	}

	sourceSprints, err := getOpenSprints(s.sourceClient, sourceBoardID)
	if err != nil {
		return err
	}

	targetSprints, err := getOpenSprints(s.targetClient, targetBoardID)
	if err != nil {
		return err
	}

	for _, sourceSprint := range sourceSprints {
		targetSprint, targetSprintFound := internal.SliceFind(targetSprints, func(targetSprint jira.Sprint) bool {
			return targetSprint.Name == sourceSprint.Name
		})

		if targetSprintFound {
			s.sourceTargetSprintMap[sourceSprint.ID] = &targetSprint
			continue
		}

		createdSprint, response, err := s.targetClient.Sprint.Create(&jira.Sprint{Name: sourceSprint.Name, OriginBoardID: targetBoardID})
		if err != nil {
			return parseResponseError("Sprint.Create", response, err)
		}

		s.sourceTargetSprintMap[sourceSprint.ID] = createdSprint

		log.Printf("Created sprint %s", sourceSprint.Name)
	}

	return nil
}

func (s *migrator) setupTargetSprint(sourceIssue *jira.Issue, targetIssue *jira.Issue) error {
	rawSourceFieldValue, ok := s.getCustomFieldValue(sourceIssue, "Sprint").([]interface{})
	if !ok || len(rawSourceFieldValue) == 0 {
		return nil
	}

	var openSourceSprint map[string]interface{}

	for _, rawSourceSprint := range rawSourceFieldValue {
		sourceSprint, ok := rawSourceSprint.(map[string]interface{})
		if !ok {
			return errors.Errorf("Could not parse source sprint")
		}

		if sourceSprint["state"] == "future" {
			openSourceSprint = sourceSprint
			break
		}
	}

	rawSourceSprintID, ok := openSourceSprint["id"]
	if !ok {
		return errors.Errorf("Source sprint ID could not be parsed")
	}

	sourceSprintID := int(rawSourceSprintID.(float64))
	targetSprint, ok := s.sourceTargetSprintMap[sourceSprintID]
	if !ok {
		return errors.Errorf("Target sprint '%s' not found", openSourceSprint["name"])
	}

	if response, err := s.targetClient.Sprint.MoveIssuesToSprint(targetSprint.ID, []string{targetIssue.ID}); err != nil {
		return parseResponseError("MoveIssuesToSprint", response, err)
	}

	return nil
}

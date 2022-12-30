package main

import (
	"bytes"
	"flag"
	"log"

	"github.com/natenho/go-jira-migrate/migration"
)

const defaultWorkerPoolSize = 8

type flagStringArray []string

func (f *flagStringArray) String() string {
	if f == nil {
		return ""
	}

	var buffer bytes.Buffer
	for _, value := range *f {
		buffer.WriteString(value)
	}

	return buffer.String()
}

func (f *flagStringArray) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	var sourceUrl = flag.String("s", "", "Source JIRA URL (e.g. https://your-source-domain.atlassian.net/)")
	var targetUrl = flag.String("t", "", "Target JIRA URL (e.g. https://your-target-domain.atlassian.net/)")
	var user = flag.String("u", "", "User")
	var apiKey = flag.String("k", "", "API Key (to create one, visit https://tinyurl.com/jira-api-token/)")
	var sourceProjectKey = flag.String("sp", "", "Source project key (e.g. MYPROJ)")
	var targetProjectKey = flag.String("tp", "", "Target project key (e.g. OTHER)")
	var jql = flag.String("q", "", "JQL query returning issues to be migrated from the selected project (e.g. \"status != Done\" to migrate only pending issues)")
	var workers = flag.Int("w", defaultWorkerPoolSize, "How many migrations should occur in parallel")
	var importSprints = flag.Bool("sprints", true, "Define if sprints will be imported, useful for projects that do not have sprints")

	var customFields flagStringArray
	flag.Var(&customFields, "cf", "Custom fields to read from source project (includes 'Story point estimate' and 'Flagged' by default)")
	customFields = append(customFields, "Story point estimate")
	customFields = append(customFields, "Story Points")
	customFields = append(customFields, "Flagged")

	var additionalLabels flagStringArray
	flag.Var(&additionalLabels, "al", "Additional labels to assign to migrated issues (includes 'MIGRATED' label) by default")
	additionalLabels = append(additionalLabels, "MIGRATED")

	flag.Parse()

	if *sourceProjectKey == "" {
		log.Println("Invalid project key")
		flag.Usage()
		return
	}

	if *targetProjectKey == "" {
		log.Println("Invalid target project key")
		flag.Usage()
		return
	}

	if *jql == "" {
		log.Println("Invalid JQL query")
		flag.Usage()
		return
	}

	migrator, err := migration.NewMigrator(
		*sourceUrl,
		*targetUrl,
		*user,
		*apiKey,
		*sourceProjectKey,
		*targetProjectKey,
		migration.WithWorkerPoolSize(*workers),
		migration.WithAdditionalLabels(additionalLabels...),
		migration.WithCustomFields(customFields...),
		migration.WithSprints(*importSprints))
	if err != nil {
		log.Println(err)
		return
	}

	results, err := migrator.Execute(*jql)
	if err != nil {
		log.Println(err)
		return
	}

	var issueCount int
	for result := range results {
		log.Println(result)
		issueCount++
	}

	log.Printf("%d issues processed.", issueCount)
}

package main

import (
	"flag"
	"log"

	"github.com/natenho/go-jira-migrate/migration"
)

const defaultWorkerPoolSize = 8

func main() {
	var sourceUrl = flag.String("s", "", "Source JIRA URL (e.g. https://your-source-domain.atlassian.net/)")
	var targetUrl = flag.String("t", "", "Target JIRA URL (e.g. https://your-target-domain.atlassian.net/)")
	var user = flag.String("u", "", "User")
	var apiKey = flag.String("k", "", "API Key (to create one, visit https://tinyurl.com/jira-api-token/)")
	var projectKey = flag.String("p", "", "Project Key")
	var jql = flag.String("q", "", "JQL query returning issues to be migrated from the selected project")
	var workers = flag.Int("w", defaultWorkerPoolSize, "How many migrations should occur in parallel")

	flag.Parse()

	if *projectKey == "" {
		log.Println("Invalid project key")
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
		migration.WithWorkerPoolSize(*workers),
		migration.WithAdditionalLabel("MIGRATED"),
		migration.WithCustomFields("Story point estimate"))
	if err != nil {
		log.Println(err)
		return
	}

	results, err := migrator.Execute(*projectKey, *jql)
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

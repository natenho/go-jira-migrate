package main

import (
	"flag"
	"log"

	"github.com/natenho/go-jira-migrate/migration"
)

const defaultWorkerPoolSize = 8

func main() {
	var sourceUrl = flag.String("s", "", "Source JIRA URL")
	var targetUrl = flag.String("t", "", "Target JIRA URL")
	var user = flag.String("u", "", "User")
	var apiKey = flag.String("k", "", "API Key")
	var projectKey = flag.String("p", "", "Project Key")
	var jql = flag.String("q", "", "JQL query returning issues to be migrated from a single project")
	var workers = flag.Int("w", defaultWorkerPoolSize, "How many migrations should occur in parallel")

	flag.Parse()

	if *projectKey == "" {
		log.Println("Invalid project key")
		flag.Usage()
		return
	}

	if *projectKey == "" {
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

	for result := range results {
		log.Println(result)
	}
}

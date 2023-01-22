 # GO JIRA Migration Tool
 [![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fnatenho%2Fgo-jira-migrate.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fnatenho%2Fgo-jira-migrate?ref=badge_shield)

This tool can migrate team-managed project issues between two JIRA Cloud accounts.

## Usage

```
  -api-key string
        API Key (to create one, visit https://tinyurl.com/jira-api-token/)
  -delete-on-error
        Define if issues migrated with errors should be deleted
  -field value
        Custom fields to read from source project (includes 'Story point estimate' and 'Flagged' by default)
  -label value
        Additional labels to assign to migrated issues (includes 'MIGRATED' label) by default
  -query string
        JQL query returning issues to be migrated from the selected project (e.g. "status != Done" to migrate only pending issues) (default "Status != Done")
  -source string
        Source JIRA URL (e.g. https://your-source-domain.atlassian.net/)
  -source-project string
        Source project key (e.g. MYPROJ)
  -sprints
        Define if sprints will be imported (default true)
  -target string
        Target JIRA URL (e.g. https://your-target-domain.atlassian.net/)
  -target-project string
        Target project key (e.g. OTHER)
  -user string
        User
  -workers int
        How many migrations should occur in parallel (default 8)
```

### Simple example

This example is a common usage scenario, migrating all pending issues.

```
/go-jira-migrate -source https://SOURCE-JIRA.atlassian.net/ -target https://TARGET-JIRA.atlassian.net/ -user your-jira-user -api-key xxxxxxxxxxxxxxxxxxx -source-project SOURCE-PROJ -target-project TARGET-PROJ -query "status != Done"
```

### Full example

This example include some additional switches and custom fields to be migrated, like issue "Story Points".

```
./go-jira-migrate -workers=8 -sprints=true -delete-on-error=true -source https://SOURCE-JIRA.atlassian.net/ -target https://TARGET-JIRA.atlassian.net/ -user your-jira-user -api-key xxxxxxxxxxxxxxxxxxx -source-project SOURCE-PROJ -target-project TARGET-PROJ -query "status != Done" -field "Story Points" -field "Start date" -field "Due date" -field "due" -field "duedate" -field "Due Data" -field "Issue color"
```

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fnatenho%2Fgo-jira-migrate.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fnatenho%2Fgo-jira-migrate?ref=badge_large)

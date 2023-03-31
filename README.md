 # GO JIRA Migration Tool
 [![Donate!](https://img.shields.io/badge/Donate-PayPal-green.svg)](https://www.paypal.com/cgi-bin/webscr?cmd=_donations&business=D5KHS5GJPJ5PQ&currency_code=BRL&source=url)
 [![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fnatenho%2Fgo-jira-migrate.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fnatenho%2Fgo-jira-migrate?ref=badge_shield)

This tool can migrate both company-managed and team-managed project issues between two JIRA Cloud accounts. It migrates attachments, description, comments, images, links, custom fields and so on by actually reading the original issue and creating a new one with the same details. The main purpose of this tool is the lack of support for migrating team-managed projects.

## References

- [CLOUD-11467 - Support the Migration of Team Managed projects](https://jira.atlassian.com/browse/CLOUD-11467)
- [What migrates in a cloud-to-cloud migration for Jira](https://support.atlassian.com/migration/docs/what-migrates-in-a-cloud-to-cloud-migration-for-jira/)

## Usage

[Download the latest binaries](https://github.com/natenho/go-jira-migrate/releases/latest) and run

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

## Recommendations

- Create a dedicated user for the migration, so it can be easily identified
- Make sure the user has Administrator access to the source and target JIRA projects
- Make sure assignees and reporters have access to the target JIRA project. The tool will do a best effort to set those.
- The target JIRA project must exist and must have the same issue types and custom fields
- Make sure that attachment upload sizes are identical between the accounts (Refer to https://support.atlassian.com/jira-cloud-administration/docs/configure-file-attachments/ to configure limits)

## Features and Limitations

- Created issues in the target project will not have the same key as the source project (even if the project keys are the same)
- Created issues are enriched with migration information, so it is easy to find a issue in the new JIRA project by the old key
- The original issue will be linked to the created issue
- Comments are all made by the migration user, mentioning the original user that wrote the comment
- Created/Updated dates are lost because all issues are created at the moment of the migration

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fnatenho%2Fgo-jira-migrate.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fnatenho%2Fgo-jira-migrate?ref=badge_large)

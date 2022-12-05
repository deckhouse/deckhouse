# Changelog Workflows

The set of workflows to generate changelogs for release messages and custom resource in clusters.
The changelog is generated on milestone assignment on merged PRs or by merging the PR or by calling
`/changelog` command in a 'Changelog PR' comments.

## Requirements

Changelog generation relies on repo secret `CHANGELOG_ACCESS_TOKEN` which must have `workflow`
permission.

## Code

The changelog toolchain consists of these files

```
.github/
    actions/
        milestone-changelog/
            action.yml
    scripts/js/
        changelog-command-validate.js
    workflows/
        changelog-command.yml
        changelog-by-milestone.yml
        changelog-by-pull.yml
        dispatch-slash-command.yml
```


## Actions
### Milestone changelog action

The action creates or updates a changelog PR for a given *open* milesone. It is used by workflows.

## Workflows

### Changelog by milestone

Generates all changelog PRs for opened milestones. Triggered by changing milestone in a merged PR.

### Changelog by pull

Generates the changelog on PR change for the PR milestone.

### Changelog command dispatch

Handles `/changelog` command and dispathes the `changelog-command` repository event.

### Changelog command

Handles the `changelog-command` in the contet of a changelog pull request, and calls the action to
update the PR in-place with fresh changelog.

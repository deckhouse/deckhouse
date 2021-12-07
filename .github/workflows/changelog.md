# Changelog Workflows

## What they do

Release changelog is generated on milestone assignment or PR merging or by calling command
`/changelog` in 'changelog PR' comments.

### Requirements

Changelog generation relies on repo secret `CHANGELOG_ACCESS_TOKEN` which must
have `workflow` permission.

### Code

The changelog toolchain consists of these files

```
.github/
    actions/
        milestone-changelog/
            action.yml
    scripts/js/
        changelog-command-validate.js
    workflows/
        changelog-command-dispatch.yml
        changelog-command.yml
        changelog.yml
```

**Milestone changelog action** creates or updates a changelog PR for a given
*open* milesone. It is used by two workflows.

**Changelog workflow** re-generates all changelog PRs on push to the main
branch.

**Changelog command dispatch** handles `/changelog` command and dispathes the
`changelog-command` repository event.

**Changelog command** handles the `changelog-command` in the contet of a
changelog pull request, and calls the action to update the PR in-place with
fresh changelog.

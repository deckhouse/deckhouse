# Workflows development

## Templates

Files in workflows directory render from 3 directories with templates: workflow_templates,
ci_templates, and ci_includes. Use render-workflows.sh to render workflows directory
after changing templates (requires Docker):

```
cd .github
./render-workflows.sh
```

## Testing

We use pull_request_target and workflow_dispatch events which require workflow file
be committed in the main branch.
There are additional repositories for workflow tests which do not interfere with ones in the main repo.

Add remotes:

```
git remote add test-1 git@github.com:deckhouse/deckhouse-test-1
git remote add test-2 git@github.com:deckhouse/deckhouse-test-2
```

Commit and push current branch to the main branch in test-1 repo:

``` 
git commit ...
git push test-1 HEAD:main --force
```

### Differences with main repo:

1. No two-repo schema for werf build.
2. Final images are pushed to ghcr.io (e.g. ghcr.io/deckhouse/deckhouse-test-1).
3. Repo for documentation and site images is ghcr.io.
4. Only AWS and static providers available for E2E tests.
5. No alerts on E2E fails in main branch.
6. Push for deploy and suspend can be skipped with SKIP_PUSH_FOR_SUSPEND and SKIP_PUSH_FOR_DEPLOY variables.
7. No registry cleanup.
8. Autoclose for Dependabot PRs (can be enabled with secret ENABLE_DEPENDABOT_IN_FORKS=true).


## Workflows schema 

Trigger -> workflow -> result.

![Workflows schema](ci-schema.png)

## Description

Kubernetes has new patch versions

## Why do we need it, and what problem does it solve?

Add support for new patch versions of kubernetes

## Why do we need it in the patch release (if we do)?

Timely update of new versions of kubernetes ensures security and fault tolerance of the cluster

## Checklist
- [x] e2e tests passed.

## Changelog entries

Add support for new patch versions of kubernetes

```changes
section: candi
type: chore
summary: Bump patch versions of Kubernetes images.
impact: Kubernetes control-plane components will restart, kubelet will restart
```

<!---
`impact_level: default` adds to changelog as usual, this is the default that can be omitted
`impact_level: high`    something important for users, the impact will be copied to "Know Before Update" section
`impact_level: low`     omitted in changelog YAML; note there is `type:chore` for chores

Tip for the section field:

  - <kebab-case of a module>, e.g. "cloud-provider-aws", "node-manager"
  - "ci", has forced low impact
  - "docs", includes website changes, should have low impact
  - "candi"
  - "deckhouse-controller"
  - "dhctl"
  - "global-hooks"
  - "go_lib"
  - "helm_lib"
  - "jq_lib"
  - "shell_lib"
  - "testing", has forced low impact
  - "tools", has forced low impact

Find changed sections:

gh pr diff   $PULL_REQUEST_NUMBER   |
  egrep "^([+]{3} b|[-]{3} a)/" |
  cut -d/ -f2- |
  sed 's#^ee/##' |
  sed 's#^fe/##' |
  sed 's#^modules/##' |
  sed 's#[0-9][0-9][0-9]-##' |
  egrep -v 'Makefile' |       # add file exclusion here
  cut -d/ -f1 |
  sort |
  uniq

Find all possible sections (excluding ci):

node -e 'console.log(require("./.github/scripts/js/changelog-find-sections.js")().join("\n"))'
-->

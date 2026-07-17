## Description
<!---
  Describe your changes in detail.

  Please let users know if your feature influences critical cluster components
  (restarts of ingress-controllers, control-plane, Prometheus, etc).
-->

## Why do we need it, and what problem does it solve?
<!---
  This is the most important paragraph.
  You must describe the main goal of your feature.

  If it fixes an issue, place a link to the issue here.

  If it fixes an obvious bug, please tell users about the impact and effect of the problem.
-->

## Why do we need it in the patch release (if we do)?

<!---
Describe why the changes need to be backported into the patch release.

If it doesn't matter whether the changes will be backported into the patch release, specify "Not necessarily".

Delete the section if the PR is for release, and not for the patch release.
-->

## Checklist
- [ ] The code is covered by unit tests.
- [ ] e2e tests passed.
- [ ] Documentation updated according to the changes.
- [ ] Changes were tested in the Kubernetes cluster manually.

## Changelog entries
<!---
  Describe the changes so they will be included in a release changelog.

  Find examples and documentation below, or visit the [Guidelines for working with PRs](https://github.com/deckhouse/deckhouse/wiki/Guidelines-for-working-with-PRs).
-->

```changes
section: <kebab-case of a module name> | <1st level dir in the repo>
type: fix | feature | chore
summary: <ONE-LINE of what effectively changes for a user>
impact: <what to expect for users, possibly MULTI-LINE>, required if impact_level is high ↓
impact_level: default | high | low
```

<!---
`impact_level: default` adds to changelog as usual, this is the default that can be omitted
`impact_level: high`    something important for users, the impact will be copied to "Know Before Update" section
`impact_level: low`     omitted in changelog YAML; note there is `type:chore` for chores
-->

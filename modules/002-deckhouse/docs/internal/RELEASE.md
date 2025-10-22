# DeckhouseRelease details

## Available annotations

| annotation                                 | description                                                                                  |
|--------------------------------------------|----------------------------------------------------------------------------------------------|
| `release.deckhouse.io/force`               | Apply specified release without any checks. Force deploy.                                    |
| `release.deckhouse.io/disruption-approved` | Approve release with disruptive changes. Works if `update.disruptionApprovalMode` is Manual. |
| `release.deckhouse.io/approved`            | Approve release for deployment. Works if `update.mode` is Manual.                            |

## Difference between disruption check and requirement check

All release settings, requirements, and disruptions are stored in the [release.yaml](release.yaml) file.

- RequirementCheck (like `"ingressNginx": "1.1"`, or `"k8s": "1.19"` in the section `requirements`) — it's some strict check of outer dependencies which deny a release deploy until a requirement is met (hard block).
- DisruptionCheck - (like `"ingressNginx"` in the section `disruptions`) — it's a check that warn a user about some meaningful changes, which are controlled by our code and logic (simple: we are changing some behavior or default value) (soft block).

### Disruptive release

It's a release with some potentially dangerous changes (e.g. which changes some default value, behavior, or changes docker to containerd, etc.).
To handle this release, you should add disruption check logic in a release at least `X-1` (previous release), for example — register DisruptionFunc in init() [example](modules/ingress-nginx/hooks/requirements.go).
And add record for a specified release, where this logic will be checked. (e.g.: `"1.36": ["ingressNginx"]` to the [release.yaml](release.yaml) file, section `disruptions`.

How to add a disruptive change:
- In the current release (say, 1.35.0) set disruption check logic into the `ingressNginx` function.
- Add the record `"1.36": ["ingressNginx"]` to the `release.yaml` file.
- In the release 1.36.0 logic function will run and disruptions will be checked.

### Release requirements

For checking some precondition/requirement — register CheckFunc like [here](modules/ingress-nginx/hooks/requirements.go)
and then add `"$functionName": "$version"` to the [release.yaml](release.yaml) file, section: `requirements`. (e.g., `"ingressNginx": "0.33"`)

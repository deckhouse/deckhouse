# DeckhouseRelease details

## Available annotations

| annotation                               | description                                                                        |
|------------------------------------------|------------------------------------------------------------------------------------|
| `release.deckhouse.io/force`               | Apply specified release without any checks. Force deploy.                          |
| `release.deckhouse.io/disruption-approved` | Approve release with disruptive changes. Works if `update.disruptionMode` is Manual. |
| `release.deckhouse.io/approved`            | Approve release for deployment. Works if `update.mode` is Manual.                    |

## Difference between disruption check and requirement check

- RequirementCheck (like `"ingressNginx": "1.1"`, or `"k8s": "1.19"`) - it's some strict check of outer dependencies which deny a release deploy until a requirement is met. (hard block)
- DisruptionCheck - (like `"disruption:ingressNginx": "true"`) - it's a check, that warn user about some meaningful changes, which are controlled by our code and logic (simple: we are changing some behavior / default value) (soft block)

### Disruptive release

It's a release with some potentially dangerous changes (change some default value / behavior / docker -> containerd, etc)
To handle this release, you should add disruption check logic in a release `X-1` (previous release), for example — register requirements. DisruptionFunc in init() [example](modules/402-ingress-nginx/hooks/requirements.go).
And then in a release `X` you should add record `"disruption:$functionName": "true"` to the [requirements.json](requirements.json) file.

Example:
- In the release 1.35.0 set disruption check logic into the `ingressNginx` function;
- In the release 1.36.0 set `"disruption:ingressNginx": "true"` to the `requirements.json` file.

### Release requirements

For checking some precondition/requirement — register CheckFunc like [here](modules/402-ingress-nginx/hooks/requirements.go)
and then add `"$functionName": "$version"` to the [requirements.json](requirements.json) file. (e.g., `"ingressNginx": "0.33"`)

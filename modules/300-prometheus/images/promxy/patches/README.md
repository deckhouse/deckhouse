## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

### 002-utf8-label-parsing.patch

Applied after `go mod vendor` to the vendored prometheus parser.

Add support for quoted label names in PromQL selectors (e.g.,
`{"storage.deckhouse.io/mount-options"!=""}`). The upstream
`jacksontj/prometheus` fork used by promxy predates Prometheus UTF-8
label name support. This patch adds the `string_identifier` grammar
rule, `newMetricNameMatcher` helper, and `shouldQuoteName` logic to
`Matcher.String()` so that quoted label names are preserved when
promxy serializes the AST back to string for forwarding to Prometheus.

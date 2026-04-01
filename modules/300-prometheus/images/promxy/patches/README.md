## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

## Vendor Patches

Applied after `go mod vendor` to the vendored prometheus parser.

### vendor-patches/001-utf8-label-parsing.patch

Add support for quoted label names in PromQL selectors (e.g.,
`{"storage.deckhouse.io/mount-options"!=""}`). The upstream
`jacksontj/prometheus` fork used by promxy predates Prometheus UTF-8
label name support. This patch adds the `string_identifier` grammar
rule and `newMetricNameMatcher` helper to the vendored PromQL parser.

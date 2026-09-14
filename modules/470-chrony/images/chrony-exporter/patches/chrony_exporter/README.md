## patches

### 001-go-mod.patch

TODO: update readme


### 002-go-mod.patch

Bumps golang.org/x/crypto to v0.55.0 and golang.org/x/net to v0.58.0 (carrying
x/sync v0.22.0, x/sys v0.47.0, x/text v0.41.0) to clear the CVEs reported
against the chrony_exporter binary, including CVE-2025-58181. Raises the go
directive to 1.25.0 and drops the toolchain line, which is what those versions
require; that is the ceiling of builder/golang-alpine in this branch, so
x/crypto v0.56.0+ (go 1.26) is out of reach here.

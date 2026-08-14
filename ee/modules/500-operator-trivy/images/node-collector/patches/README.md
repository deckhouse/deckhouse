# Patches

### 001-go-mod.patch

Updates vulnerable dependencies to mitigate CVEs, including:
- CVE-2026-25681 (`golang.org/x/net` -> `v0.56.0`)
- CVE-2026-39824 (`golang.org/x/sys` -> `v0.46.0`)
- CVE-2026-56852 (`golang.org/x/text` -> `v0.39.0`)

### 002-log-commands-error.patch

Added missing error check to output real error during `node-commands` flag parsing.

# 001-support-legacy-annotation.patch

Support extended-monitoring legacy annotation for now. Upstream project has an option to use label Namespace selector only.

# 002-fix-cves.patch

Fix CVE-2025-15558 (`github.com/docker/cli`).
```sh
go get github.com/docker/cli@v29.2.0
go mod tidy
```

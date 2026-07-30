# 001-support-legacy-annotation.patch

Support extended-monitoring legacy annotation for now. Upstream project has an option to use label Namespace selector only.

# 002-go-mod.patch

Bump `golang.org/x/net` to v0.56.0, `golang.org/x/sys` to v0.46.0 and `golang.org/x/text` to v0.39.0 (and
their transitive `golang.org/x/term`, `golang.org/x/sync`) to fix CVEs reported by Trivy against the
embedded Go dependencies (CVE-2025-47911, CVE-2025-58190, CVE-2026-25680/25681/27136/33814/39821/42502/42506/46600,
CVE-2026-39824, CVE-2026-56852).

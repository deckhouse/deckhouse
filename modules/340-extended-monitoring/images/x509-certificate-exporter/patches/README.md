### 001-go-mod.patch

Update dependencies. Bumps `golang.org/x/crypto` to v0.53.0, `golang.org/x/net` to v0.56.0,
`golang.org/x/sys` to v0.46.0 and `golang.org/x/text` to v0.39.0 (and transitive `x/sync`, `x/term`)
to fix Trivy-reported CVEs in the embedded Go dependencies.

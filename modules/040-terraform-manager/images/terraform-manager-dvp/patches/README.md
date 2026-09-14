# 001-add-config-data-base64.patch
Add argument config_data_base64 to the provider configuration

# 002-go-mod.patch
Bumps golang.org/x/crypto to v0.55.0 and golang.org/x/net to v0.58.0 (carrying
x/sys v0.47.0, x/text v0.41.0, x/term v0.45.0, x/mod v0.38.0, x/tools v0.48.0,
x/sync v0.22.0) to clear the CVEs reported against the terraform-provider-kubernetes
binary in the cse-1.73 scan. The go directive stays at 1.25.0, the ceiling of
builder/golang-alpine in this branch; x/crypto v0.56.0+ needs go 1.26 and is out
of reach here. Regenerate with `go get golang.org/x/net@<v> golang.org/x/crypto@<v>
&& go mod tidy` on a clean v2.37.1 checkout with 001 applied, then `git diff -- go.mod go.sum`.

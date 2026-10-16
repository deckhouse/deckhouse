# Patches

## 001-protoc-gen-go-json-version.patch

Allow setting the CLI version with `-ldflags=-X main.buildVersion=...` when
building protoc-gen-go-json v1.6.0 from Git sources exported without `.git`.
The recipe shares one version variable between `git.tag` and the linker flag.
This does not restore
Go module versions or `vcs.*` fields in Go build info.

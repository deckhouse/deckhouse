## Patches

## 001-go-mod.patch

Bump go.mod dependencies to fix known CVEs.

## 002-replace-gofsutil.patch

Replaces `github.com/akutz/gofsutil` with the local Deckhouse fork used during image build.

## 003-fetch-hosts-by-datastore.patch

Adds support for fetching attached hosts when topology is resolved from a `Datastore`.

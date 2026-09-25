## Patches

## 001-go-mod.patch

Bump go.mod dependencies to fix known CVEs.

## 003-fetch-hosts-by-datastore.patch

Adds support for fetching attached hosts when topology is resolved from a `Datastore`.


## 002-gofsutil-local-fork.patch

Restore the `replace github.com/akutz/gofsutil => ../gofsutil` directive in go.mod.
werf.inc.yaml rewrites that path with sed before the build, so it must be present.
Kept as a separate patch so regenerating 001-go-mod.patch cannot drop it.

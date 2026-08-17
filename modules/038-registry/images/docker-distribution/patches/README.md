## Patches

### 001-skip-mode-cleanup.patch

Add a `proxy.skipmodecleanup` option that keeps the store when the registry starts in a mode other
than the one that last wrote it, and leave the option off by default.

The fork deletes everything under `/docker` on a transition between cache and non-cache modes,
deciding which mode last wrote the directory by whether the proxy scheduler's state file is present.
That holds for a registry that owns its directory. This module runs two registries over one store —
one proxying for reads, one accepting the writes that fill it — and patches the expiry scheduler out
at build time (see `scripts/no-proxy-expiry.sh`), so the state file appears and disappears for reasons
unrelated to the data. Each start of the non-proxying instance then read as a mode change.

Measured on a three-master cluster: 3236 files and twelve gigabytes deleted two seconds after the
write instance came up, twice in one afternoon, and once at the exact moment the upstream was dropped
for an air-gap — leaving the cluster with no images and nowhere to pull them from.

The option is set for both instances by the syncer. Default behaviour is unchanged, so the old
in-cluster proxy implementation, which shares this image, is not affected.

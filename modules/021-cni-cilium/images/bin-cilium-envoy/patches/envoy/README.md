# Patches

Currently there are no Envoy patches.

## 001-fix-cve.patch (removed)

Previously bumped vulnerable Python build-tooling dependencies in
`tools/base/requirements.{in,txt}` (aiohttp, jinja2, protobuf, requests,
setuptools, urllib3, cryptography, etc.).

Removed during the upgrade to Envoy v1.36.8: upstream already pins all of
these dependencies at fixed (non-vulnerable) versions, so the patch became
fully redundant and no longer applied.

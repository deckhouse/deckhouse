# Patches

## 002-hosts-rewrite.patch
Adds ability to rewrite path (repository) part for mirror defined in containerd [host](https://github.com/containerd/containerd/blob/v1.7.24/docs/hosts.md) configuration.
Configuration will be applied wihout containerd service restart.

To use this support of dynamic hosts configuration must be enabled in containerd by specifying following setting:

```toml
[plugins."io.containerd.grpc.v1.cri".registry]
config_path = "/etc/containerd/registry.d"
```

*Warning: after specify this setting you will disable [deprecated legacy mirrors](https://github.com/containerd/containerd/blob/v1.7.24/docs/cri/registry.md#configure-registry-endpoint) defined by `[plugins."io.containerd.grpc.v1.cri".registry.mirrors]`*


Usage example:

`/etc/containerd/registry.d/registry.deckhouse.local/hosts.toml`
```toml
server = "https://dev-registry.deckhouse.io"
capabilities = ["pull", "push", "resolve"]

# first rewrite with regex match will be applied
# check will be in order as defined in this configuration file
[[rewrite]]
regex = "^/system/deckhouse"
replace = "/sys/deckhouse-oss"

# repos starts with /system/deckhouse will be matched by previous rewrite
# so it will be not handled by this rewrite 
[[rewrite]]
regex = "^/system/"
replace = "/sys-other/"

# Capture groups can be used in replace in 
# form specified in https://pkg.go.dev/regexp#Regexp.Expand
[[rewrite]]
regex = "^/test/(.+)"
replace = "/other/${1}/service"

[host]

  # Also it may be configured if multiple hosts specified
  [host."dev-registry2.deckhouse.io"]
  capabilities = ["pull", "resolve"]

    [[host."dev-registry2.deckhouse.io".rewrite]]
    regex = "^/system/"
    replace = "/sys-registy2/"

    [[host."dev-registry2.deckhouse.io".rewrite]]
    regex = "^/system/"
    replace = "/sys-other2/"

  [host."dev-registry3.deckhouse.io"]
  capabilities = ["pull", "resolve"]

    [[host."dev-registry3.deckhouse.io".rewrite]]
    regex = "^/system/"
    replace = "/sys-registy3/"

    [[host."dev-registry3.deckhouse.io".rewrite]]
    regex = "^/system/"
    replace = "/sys-other3/"

```

## 003-hosts-auth.patch

Adds ability to specify authentication credentials for mirror defined in containerd [host](https://github.com/containerd/containerd/blob/v1.7.24/docs/hosts.md) configuration.
Configuration will be applied wihout containerd service restart.

To use this support of dynamic hosts configuration must be enabled in containerd by specifying following setting:

```toml
[plugins."io.containerd.grpc.v1.cri".registry]
config_path = "/etc/containerd/registry.d"
```

*Warning:*
- *after specify this setting you will disable [deprecated legacy mirrors](https://github.com/containerd/containerd/blob/v1.7.24/docs/cri/registry.md#configure-registry-endpoint) defined by `[plugins."io.containerd.grpc.v1.cri".registry.mirrors]`*
- *it also will disable usage of the [deprecated `registry.configs.*.auth`](https://github.com/containerd/containerd/blob/main/docs/cri/registry.md#configure-registry-credentials) sections in main config file **(it is differs from standart containerd behavior)***

Usage example:

`/etc/containerd/registry.d/registry.deckhouse.local/hosts.toml`
```toml
server = "https://dev-registry.deckhouse.io"
capabilities = ["pull", "push", "resolve"]

# Auth for top-level mirror
[auth]
username = "license-token"
password = "<my-registry-token>

[host]

  # Also it may be configured for fallback hosts
  [host."dev-registry2.deckhouse.io"]
  capabilities = ["pull", "resolve"]

    [host."dev-registry2.deckhouse.io".auth]
    username = "license-token"
    password = "<my-registry-token2>

  [host."dev-registry3.deckhouse.io"]
  capabilities = ["pull", "resolve"]

    [host."dev-registry3.deckhouse.io".auth]
    username = "license-token"
    password = "<my-registry-token2>

```

Any options supported by [deprecated `registry.configs.*.auth`](https://github.com/containerd/containerd/blob/main/docs/cri/registry.md#configure-registry-credentials) are supported in `[auth]` section

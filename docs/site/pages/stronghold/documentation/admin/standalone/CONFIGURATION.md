---
title: "Configuration"
permalink: en/stronghold/documentation/admin/standalone/configuration.html
---

## Configuration overview

Stronghold servers are configured using a configuration file in either HCL or JSON format.

To enhance control over file access, you can enable file permission checks by setting the `VAULT_ENABLE_FILE_PERMISSIONS_CHECK` environment variable.  
When this check is enabled, Stronghold verifies that the configuration directory and files are owned by the user running Stronghold.  
It also ensures that neither the group nor other users have write or execute permissions on these files.

If necessary, the operator can specify the user and file permissions for the plugin directory and executable files.  
This is done using the `plugin_file_uid` and `plugin_file_permissions` parameters in the configuration.  
By default, file permission checks in Stronghold are disabled.

Here is an example configuration:

```console
ui            = true
cluster_addr  = "https://127.0.0.1:8201"
api_addr      = "https://127.0.0.1:8200"
disable_mlock = true

storage "raft" {
  path = "/path/to/raft/data"
  node_id = "raft_node_id"
}

listener "tcp" {
  address       = "127.0.0.1:8200"
  tls_cert_file = "/path/to/full-chain.pem"
  tls_key_file  = "/path/to/private-key.pem"
}

telemetry {
  statsite_address = "127.0.0.1:8125"
  disable_hostname = true
}
```

To apply new parameters after modifying the configuration file, you need to restart the Stronghold service:  
`systemctl restart stronghold`.

### Parameter overview

* `storage` — **required** block. Configures the storage backend where Stronghold data will be stored.  
  To run Stronghold in High Availability (HA) mode, you must use a backend that supports coordination semantics.  
  If the selected storage backend supports this, HA parameters can be specified directly within the `storage` block.  
  Otherwise, you should configure a separate `ha_storage` parameter with a backend that supports HA, along with the corresponding HA parameters.  
  Details on the storage backend parameters are in the [storage section](#storage).

* `ha_storage` — optional block. Configures the storage backend where Stronghold coordination in High Availability (HA) mode will occur.  
  The specified backend must support HA. If this parameter is not set, Stronghold will attempt to run HA on the backend specified in the `storage` parameter.  
  If the storage backend already supports HA coordination and the specific HA parameters are already specified in the `storage` block, additional `ha_storage` configuration is not required.

* `listener` — **required** block. Configures the parameters for listening to Stronghold API requests.  
  Details are in the [listener section](#listener).

* `user_lockout` — optional block. Configures the behavior for user lockout after failed login attempts.  
  Details are in the [user_lockout section](#user_lockout).

* `cluster_name` — optional string. Specifies the identifier for the Stronghold cluster. If no value is provided, Stronghold will generate one.

* `cache_size` — optional string. Defines the size of the read cache used by the physical storage subsystem.  
  The value is specified as the number of stored records, so the total cache size depends on record size. Default is `131072`.

* `disable_cache` — optional boolean. Disables all caches in Stronghold, including the read cache used by the physical storage subsystem.  
  This significantly impacts performance. Default is `false`.

* `disable_mlock` — optional boolean. Disables the server’s ability to perform the `mlock` system call, which prevents memory from being paged to disk.

  It is **not recommended** to disable `mlock` unless using the integrated storage.  
  When disabling `mlock`, you should follow the additional security measures described below.

  It is **not recommended** to disable `mlock` if the systems running Stronghold either do not use swap or use only encrypted swap.  
  Memory lock support is available only on UNIX-like systems such as Linux or FreeBSD that support the `mlock()` system call.  
  Systems like Windows, NaCL, and Android do not have mechanisms to prevent the entire process address space from being written to disk, so this feature is automatically disabled for unsupported platforms.

  On the contrary, it is **strongly recommended** to disable `mlock` when using integrated storage.  
  This system call works poorly with memory-mapped files, such as those created by BoltDB, which Raft uses for state tracking.

  Using `mlock` with integrated storage can cause memory shortages if Stronghold’s data volume exceeds available RAM.  
  Memory-mapped files are loaded into resident memory, causing all Stronghold data to be loaded into RAM.  
  In this case, even though BoltDB data remains encrypted at rest, swap should be disabled to prevent other sensitive Stronghold data in memory from being paged to disk.

  On Linux, you can allow the Stronghold executable to use the `mlock` system call without running the process as root by executing the following command:

  ```shell
  sudo setcap cap_ipc_lock=+ep $(readlink -f $(which stronghold))
  ```

  Each plugin runs as a separate process, so you need to apply similar settings for each plugin in the `plugins` directory.  
  If you are using a Linux distribution with a current version of `systemd`, you can add the following directive to the `[Service]` section of the configuration file:

  ```console
  LimitMEMLOCK=infinity
  ```

* `plugin_directory` — optional string parameter. Defines the directory from which plugins are allowed to be loaded. For successful plugin loading, Stronghold must have read permissions for the files in this directory, and the value cannot be a symbolic link. By default, this parameter is not set and has the value `""`.

* `plugin_tmpdir` — optional string parameter. Specifies the directory where Stronghold can create temporary files to support interaction with Unix sockets for containerized plugins. If this value is not set, Stronghold will use the default temporary file directory. In most cases, this parameter does not need to be configured, except when containerized plugins are used and Stronghold shares the temporary folder with other processes, such as when using the `PrivateTmp` parameter in systemd.

* `plugin_file_uid` — optional integer parameter. User ID (UID) for plugin directories and executable files in case they belong to a user different from the one running Stronghold.

* `plugin_file_permissions` — optional string parameter. A string of octal permissions for plugin directories and executable files in case write or execute permissions are set for the group or other users.

* `telemetry` — optional block of parameters. Specifies the telemetry system for collecting and sending statistics.

* `default_lease_ttl` — optional string parameter. Defines the default lease TTL for tokens and secrets. The value is specified using a time suffix, such as `"40s"` (40 seconds) or `"1h"` (1 hour), and cannot exceed `max_lease_ttl`. By default, `768h`.

* `max_lease_ttl` — optional string parameter. Defines the maximum lease TTL for tokens and secrets. The value is specified using a time suffix, such as `"40s"` (40 seconds) or `"1h"` (1 hour). Specific mount points can override this value by configuring the mount point using the `max-lease-ttl` flag in the `auth` or `secret` commands. By default, `768h`.

* `default_max_request_duration` — optional string parameter. Specifies the maximum default request duration after which Stronghold will cancel it. The value is specified using a time suffix, such as `"40s"` (40 seconds) or `"1h"` (1 hour), and can be overridden for each listener through the `max_request_duration` parameter. By default, `90s`.

* `detect_deadlocks` — optional string parameter. A comma-separated string of values. Specifies internal mutually exclusive locks to watch for potential deadlocks. Currently supported values are `statelock`, `quotas`, and `expiration`, which will log `"POTENTIAL DEADLOCK:"` if a kernel state lock attempt seems to be blocked. Enabling `detect_deadlocks` may negatively impact performance due to tracking each lock attempt. By default, this parameter is not set and has the value `""`.

* `raw_storage_endpoint` — optional boolean parameter. Activates the `sys/raw` endpoint with elevated privileges. This endpoint allows decryption and encryption of raw data coming in and out of the security barrier. By default, `false`.

* `introspection_endpoint` — optional boolean parameter. Activates the `sys/internal/inspect` endpoint, which allows inspection of specific subsystems within Stronghold by users with root token or sudo privileges. By default, `false`.

* `ui` — optional boolean parameter. Activates the built-in web user interface, available on all listeners (address + port) at the path `/ui`. When browsers access the default Stronghold API address, they will automatically be redirected to the UI. By default, `false`. Details are in the [ui section](#ui-section).

* `pid_file` — optional string parameter. Specifies the path to the file where the Stronghold process ID (PID) should be stored.

* `enable_response_header_hostname` — optional boolean parameter. Activates the addition of the `X-Vault-Hostname` HTTP header to all Stronghold HTTP responses. This header will contain the hostname of the Stronghold node that handled the HTTP request. The presence of this information is not guaranteed — it will be provided if possible. If the option is enabled but the `X-Vault-Hostname` header is missing from the response, it may indicate an error extracting the hostname from the operating system. By default, `false`.

* `enable_response_header_raft_node_id` — optional boolean parameter. Activates the addition of the `X-Vault-Raft-Node-ID` HTTP header to all Stronghold HTTP responses. If Stronghold uses integrated storage (i.e., is part of a Raft cluster), the `X-Vault-Raft-Node-ID` header will contain the Raft node ID that handled the HTTP request. If the Stronghold node is not part of a Raft cluster, this header will be omitted regardless of whether the option is enabled. By default, `false`.

* `log_level` — optional string parameter. Defines the level of log detail. The following five values are supported in decreasing order of detail: `trace`, `debug`, `info`, `warn`, and `error`. This value can also be set via the environment variable `VAULT_LOG_LEVEL`. By default, `info`.

  If a valid `log_level` value is provided, when SIGHUP (`sudo kill -s HUP pid Stronghold`) is sent, Stronghold will update the existing log level. Both the CLI flag and the environment variable will be overridden. Not all parts of the Stronghold log may dynamically update. For example, secret/auth plugins are currently not dynamically updated.

* `log_format` — optional string parameter. The log format. Two values are supported: `standard` and `json`. By default, `standard`.

* `log_file` — optional string parameter. The absolute path where Stronghold should save log messages in addition to other existing outputs such as journald/stdout. Paths that end with a path separator use the default file name — `vault.log`. Paths that don't end with an extension use the default extension — `.log`. If the log file is overwritten, Stronghold appends the current timestamp to the file name at the time of overwriting.

* `log_rotate_duration` — optional string parameter. Specifies the maximum duration of writing to the log file before it is overwritten. The value is specified using a time suffix, such as `40s`. By default, `24h`.

* `log_rotate_bytes` — optional integer parameter. Specifies the number of bytes that can be written to the log file before it is overwritten. If not specified, the number of bytes written to the log file is unlimited.

* `log_rotate_max_files` — optional integer parameter. Specifies the maximum number of old log files to keep. By default, the value is `0`, meaning files are never deleted. To delete old log files when creating a new one, set the value to `-1`.

* `imprecise_lease_role_tracking` — optional boolean parameter. Allows skipping the lease count by roles if role-based quotas are not enabled. When set to `true` and new role-based quotas are enabled, the subsequent lease count will start at 0. This parameter affects role-based lease quotas but reduces latency when role quotas are not used.

* `experiments` — optional array of values. A list of experimental features to activate for the node. Do not use experimental features in production environments! Associated APIs may undergo incompatible changes between releases. Additional experimental features can also be specified through the environment variable `VAULT_EXPERIMENTS` as a comma-separated list of values.

### High availability parameters

For backends that support High Availability (HA) mode, the following parameters are used:

* `api_addr` — optional string parameter. Specifies the address that will be advertised to other Stronghold servers in the cluster for client redirection. This value is also used for plugin backends. The `api_addr` parameter can also be set through the environment variable `VAULT_API_ADDR`. Generally, this should be a full URL pointing to the listener's address. The address can be dynamically determined using the [go-sockaddr template](https://pkg.go.dev/github.com/hashicorp/go-sockaddr/template), which is resolved at runtime.

* `cluster_addr` — optional string parameter. Specifies the address to be advertised to other Stronghold servers in the cluster for request redirection. This parameter can also be set through the environment variable `VAULT_CLUSTER_ADDR`. Similar to `api_addr`, this is a full URL, but Stronghold will ignore the scheme (all cluster members always use TLS with a private key/certificate). The address can be dynamically determined using the [go-sockaddr template](https://pkg.go.dev/github.com/hashicorp/go-sockaddr/template), which is resolved at runtime.

* `disable_clustering` — optional boolean parameter. Specifies whether clustering features, such as request forwarding, are enabled. Setting the value to `true` for one storage node will disable clustering features only if that node is active. The parameter cannot be set to `true` if the storage type is `raft`. By default, `false`.

## Listener section {#listener}

The `listener` section configures the addresses and ports where Stronghold will listen for requests. Currently, there are two types of listeners:

1. TCP.
1. Unix Domain Socket.

### TCP

The TCP listener configures Stronghold to listen on a TCP address/port.

```console
listener "tcp" {
  address = "127.0.0.1:8200"
}
```

You can specify the `listener` section multiple times to have Stronghold listen on multiple interfaces. When configuring multiple listeners, be sure to set the `api_addr` and `cluster_addr` parameters so that Stronghold advertises the correct address to other nodes.

#### Hiding Confidential Data for Unauthenticated Endpoints

Unauthenticated API endpoints may return the following confidential information:

1. The Stronghold version number.
1. The build date of the Stronghold binary.
1. The name of the Stronghold cluster.
1. The IP address of nodes in the cluster.

Stronghold allows configuring each `tcp` `listener` section to remove this data from API responses if necessary. The removal of sensitive information based on the listener section configuration is supported for the following three API endpoints:

1. `/sys/health`
1. `/sys/leader`
1. `/sys/seal-status`

The removed information is replaced by an empty string `""`. Additionally, some Stronghold API responses omit keys if the corresponding value is empty (`""`).

{% alert level="warning" %}
Removing values affects responses for all API clients. The Stronghold command-line interface (CLI) and user interface use Stronghold API responses. As a result, the removal settings will apply to the output in the CLI and user interface, as well as to direct API calls.
{% endalert %}

### Custom Response Headers

Stronghold allows setting custom HTTP response headers for the root path (`/`) and for API endpoints (`/v1/`). These headers are determined based on the returned status code. For example, a user can define one set of custom response headers for the `200` status code and another set for the `307` status code.

The `/sys/config/ui` API endpoint allows users to set UI-specific custom headers. However, if the header is configured in the configuration file, it cannot be reconfigured through this endpoint. To remove or modify a custom header, you need to update the Stronghold configuration file and send the `SIGHUP` signal to the Stronghold process.

If a custom header is set in the configuration file and the same header is used by internal Stronghold processes, the configured header will not be accepted. For example, a custom header with the `X-Vault-` prefix will not be accepted. A corresponding log message will be registered in the Stronghold logs.

#### Priority Order

If the same header is specified both in the configuration file and via the `/sys/config/ui` API endpoint, the header from the configuration file takes priority. For example, the `Content-Security-Policy` header is defined by default in the `/sys/config/ui` endpoint. However, if the same header is specified in the configuration file, Stronghold will use it and substitute it in the response instead of the default value from `/sys/config/ui`.

#### TCP Listener Parameters

* `address` — string parameter in the format `ip-address:port`. Specifies the address and port to listen on. The parameter can be dynamically determined using the [go-sockaddr template](https://pkg.go.dev/github.com/hashicorp/go-sockaddr/template), which is resolved at runtime.

* `cluster_address` — string parameter in the format `ip-address:port`. Specifies the address and port to listen for server-to-server requests in the cluster. By default, the value of this parameter is one port higher than the `address` value. In most cases, you don’t need to set `cluster_address`, but the parameter can be useful if isolated Stronghold servers need to skip a TCP load balancer or use another connection scheme. The parameter can be dynamically determined using the [go-sockaddr template](https://pkg.go.dev/github.com/hashicorp/go-sockaddr/template), which is resolved at runtime.

* `http_idle_timeout` — string parameter. Specifies the maximum time to wait for the next request when the keep-alive mode is enabled. The value is specified with a time suffix, such as `"40s"` (40 seconds) or `"1h"` (1 hour). If the parameter value is zero, the `http_read_timeout` value will be used. If both values are zero, the `http_read_header_timeout` value will be used.

* `http_read_header_timeout` — string parameter. Specifies the time allocated for reading the request headers. The value is specified with a time suffix, such as `"40s"` (40 seconds) or `"1h"` (1 hour).

* `http_read_timeout` — string parameter. Specifies the maximum duration to read the entire request, including headers and body. The value is specified with a time suffix, such as `"40s"` (40 seconds) or `"1h"` (1 hour).

* `http_write_timeout` — string parameter. Specifies the maximum duration to write the response. It is reset every time a new request header is read. By default, `0` means no time limit. The value is specified with a time suffix, such as `"40s"` (40 seconds) or `"1h"` (1 hour).

* `max_request_size` — integer parameter. Specifies the maximum allowed size for a request in bytes. By default, it is set to 32 MB if not specified or set to `0`. A value less than `0` disables the limit.

* `max_request_duration` — string parameter. Specifies the maximum time to process a request after which Stronghold will cancel it. This parameter overrides the `default_max_request_duration` for this listener. The value is specified with a time suffix, such as `"90s"` (90 seconds).

* `proxy_protocol_behavior` — string parameter. Enables PROXY protocol version 1 behavior for the listener. The following values are accepted:
  * `use_always` — always uses the client IP address.
  * `allow_authorized` — uses the client IP address if the source IP is in the list.
  * `proxy_protocol_authorized_addrs` — uses the source IP if it is not in the list.
  * `deny_unauthorized` — if the source IP address is not in the `proxy_protocol_authorized_addrs` list, traffic will be rejected.

* `proxy_protocol_authorized_addrs` — string parameter or array of strings. Specifies the list of allowed source IP addresses for use with the PROXY protocol. It is not required if `proxy_protocol_behavior` is set to `use_always`. When specified as a string, values should be comma-separated. The value `proxy_protocol_authorized_addrs` cannot be empty; at least one source IP address must be specified.

* `redact_addresses` — boolean parameter. Hides the values of `leader_address` and `cluster_leader_address` in the corresponding API responses when set to `true`.

* `redact_cluster_name` — boolean parameter. Hides the value of `cluster_name` in the corresponding API responses when set to `true`.

* `redact_version` — boolean parameter. Hides the values of `version` and `build_date` in the corresponding API responses when set to `true`.

* `tls_disable` — boolean parameter. Specifies whether TLS is disabled. Stronghold uses TLS by default, so it is necessary to explicitly disable TLS if insecure communication is not desired. Disabling TLS may cause some UI features to be disabled.

* `tls_cert_file` — string parameter. Specifies the path to the TLS certificate file. The file must be in PEM format. To configure the listener to use the CA certificate, the primary certificate and the CA certificate should be concatenated. The path specified in the `tls_cert_file` parameter is used when starting Stronghold. Changing this value during Stronghold operation will have no effect.

* `tls_key_file` — string parameter. Specifies the path to the private key file for the certificate. The file must be in PEM format. If the key file is encrypted, a passphrase will be required when starting the server. When reloading the configuration via `SIGHUP`, the passphrase between key files must remain the same. The path specified in the `tls_key_file` parameter is used when starting Stronghold. Changing this value during Stronghold operation will have no effect.

* `tls_min_version` — string parameter. Specifies the minimum TLS version supported by the listener. Accepted values: `"tls10"`, `"tls11"`, `"tls12"`, or `"tls13"`. TLS 1.1 and below (i.e., `tls10` and `tls11`) are considered insecure and not recommended.

* `tls_max_version` — string parameter. Specifies the maximum TLS version supported by the listener. Accepted values: `"tls10"`, `"tls11"`, `"tls12"`, or `"tls13"`. TLS 1.1 and below (i.e., `tls10` and `tls11`) are considered insecure and not recommended.

* `tls_cipher_suites` — string parameter. Specifies the list of supported cipher suites as a comma-separated list of values. A list of all available cipher suites can be found in the [Go TLS documentation](https://go.dev/src/crypto/tls/cipher_suites.go).

  Go uses the list specified in the `tls_cipher_suites` parameter only for TLSv1.2 and earlier versions. The order of ciphers is not important. To optimize `tls_cipher_suites`, set `tls_max_version` to `"tls12"` to prevent the negotiation of TLSv1.3. More details about this and other related TLS changes can be found in the [official Go TLS post](https://go.dev/blog/tls-cipher-suites).

* `tls_require_and_verify_client_cert` — boolean parameter. Enables client authentication for the listener. The listener will require a presented client certificate, successfully verified against system CAs.

* `tls_client_ca_file` — string parameter. A PEM-encoded certificate file for the CA used for client authentication.

* `tls_disable_client_certs` — boolean parameter. Disables client authentication for the listener. By default, `false` — Stronghold requests client authentication certificates when available.

  > **Warning.** The `tls_disable_client_certs` and `tls_require_and_verify_client_cert` fields in the `listener` section are mutually exclusive. Ensure that both are not set to `true`. By default, client certificate verification is not required.

* `x_forwarded_for_authorized_addrs` — string parameter. Can be specified as a comma-separated list of values or as a JSON array. Specifies the list of allowed source IP addresses for use with the `X-Forwarded-For` header. Enables support for `X-Forwarded-For`.

  For example, Stronghold receives a connection from the load balancer IP `1.2.3.4`. Adding `1.2.3.4` to the `x_forwarded_for_authorized_addrs` parameter will ensure that the client IP address initiating the connection, e.g., `3.4.5.6`, appears in the `remote_address` field of the audit log. It is important that the load balancer sends the client IP address in the `X-Forwarded-For` header.

* `x_forwarded_for_hop_skips` — string parameter. Specifies the number of addresses to skip from the end of the hop list. For example, if the `X-Forwarded-For` header contains addresses `1.2.3.4, 2.3.4.5, 3.4.5.6, 4.5.6.7`, and the `x_forwarded_for_hop_skips` parameter is set to `"1"`, the client IP address used will be `3.4.5.6`.

* `x_forwarded_for_reject_not_authorized` — boolean parameter. When set to `false`, it allows ignoring the `X-Forwarded-For` header if it comes from an unauthorized address. In this case, the client connection will be used as is, instead of being rejected.

* `x_forwarded_for_reject_not_present` — boolean parameter. When set to `false`, it allows using the client address as is if the `X-Forwarded-For` header is missing or empty, instead of rejecting the client connection.

* `disable_replication_status_endpoints` — boolean parameter. When set to `true`, it disables replication status endpoints for this listener.

Telemetry parameters:

* `unauthenticated_metrics_access` — boolean parameter. When set to `true`, it allows unauthenticated access to the `/v1/sys/metrics` endpoint.

Profiling parameters:

* `unauthenticated_pprof_access` — boolean parameter. When set to `true`, it allows unauthenticated access to the `/v1/sys/pprof` endpoint.

Inflight requests logging parameters:

* `unauthenticated_in_flight_requests_access` — boolean parameter. When set to `true`, it allows unauthenticated access to the `/v1/sys/in-flight-req` endpoint.

Custom response headers parameters:

A list of mappings of type:

```json
{
  "key1" = ["value1", "value 2", ...],
  "key2" = ["value1", "value 2", ...],
}
```

allows mapping default header names to an array of values. Default headers are set across all endpoints regardless of the status code value.

The list of mappings is of the type:

```json
{
  "key1" = ["value1", "value 2", ...],
  "key2" = ["value1", "value 2", ...],
}
```

allows mapping header names to an array of values. The headers specified in this section are set for the specified status codes.

The list of mappings is of the type:

```json
{
  "key1" = ["value1", "value2", ...],
  "key2" = ["value1", "value2", ...],
}
```

allows mapping header names to an array of values. The headers specified in this section are set for status codes that fall into the specified status code groups.

#### Example configuration for TCP listener section

**Example 1.** Specifying required parameters for TLS.

This demonstrates specifying the certificate and key for TLS.

```console
listener "tcp" {
  tls_cert_file = "/etc/certs/tls.crt"
  tls_key_file  = "/etc/certs/tls.key"
}
```

**Example 2.** Listening on multiple interfaces.

This shows how to listen on a private interface and localhost for Stronghold.

```console
listener "tcp" {
  address = "127.0.0.1:8200"
}

listener "tcp" {
  address = "10.0.0.5:8200"
}

# Advertise the non-loopback interface
api_addr = "https://10.0.0.5:8200"
cluster_addr = "https://10.0.0.5:8201"
```

**Example 3.** Allowing unauthenticated access to metrics.

```console
listener "tcp" {
  telemetry {
    unauthenticated_metrics_access = true
  }
}
```

**Example 4.** Allowing unauthenticated access to profiling.

```console
listener "tcp" {
  profiling {
    unauthenticated_pprof_access = true
    unauthenticated_in_flight_request_access = true
  }
}
```

**Example 5.** Configuring custom HTTP response headers.

Operators can configure the `custom_response_headers` subsection in the `listener` section to add custom HTTP headers relevant to their applications.

```console
listener "tcp" {
  custom_response_headers {
    "default" = {
      "Strict-Transport-Security" = ["max-age=31536000","includeSubDomains"],
      "Content-Security-Policy" = ["connect-src https://clusterA.vault.external/"],
      "X-Custom-Header" = ["Custom Header Default Value"],
    },
    "2xx" = {
      "Content-Security-Policy" = ["connect-src https://clusterB.vault.external/"],
      "X-Custom-Header" = ["Custom Header Value 1", "Custom Header Value 2"],
    },
    "301" = {
      "Strict-Transport-Security" = ["max-age=31536000"],
      "Content-Security-Policy" = ["connect-src https://clusterC.vault.external/"],
    },
  }
}
```

Examples of custom HTTP headers — `Strict-Transport-Security` and `Content-Security-Policy`. These can be configured to enhance the security of the application interacting with Stronghold endpoints. Vulnerability scanners often look for such security-related HTTP headers. It is also possible to configure application-specific custom headers, as shown with `X-Custom-Header` in the example above.

If a header is defined in multiple status code subsections, the header corresponding to the most specific response code will be returned. From the configuration example below, the `306` response will return the `Custom` header for 3xx, while `307` will return the `Custom` header for 307.

```console
listener "tcp" {
  custom_response_headers {
    "default" = {
       "X-Custom-Header" = ["default Custom header value"]
    },
    "3xx" = {
       "X-Custom-Header" = ["3xx Custom header value"]
    },
    "307" = {
       "X-Custom-Header" = ["307 Custom header value"]
    }
  }
}
```

**Example 6.** Listening on all IPv4 and IPv6 interfaces

In this example, Stronghold listens on all IPv4 and IPv6 interfaces, including localhost.

```console
listener "tcp" {
  address         = "[::]:8200"
  cluster_address = "[::]:8201"
}
```

**Example 7.** Listening on specific IPv6 addresses

In this example, it is configured to use only IPv6, bound to the interface with IP address `2001:1c04:90d:1c00:a00:27ff:fefa:58ec`.

```console
listener "tcp" {
  address         = "[2001:1c04:90d:1c00:a00:27ff:fefa:58ec]:8200"
  cluster_address = "[2001:1c04:90d:1c00:a00:27ff:fefa:58ec]:8201"
}
```

Declaring a non-loopback interface:

```console
api_addr = "https://[2001:1c04:90d:1c00:a00:27ff:fefa:58ec]:8200"
cluster_addr = "https://[2001:1c04:90d:1c00:a00:27ff:fefa:58ec]:8201"
```

#### Example of hiding information

**Example 1.** Configuration using `redact_addresses`, `redact_cluster_name`, and `redact_version` to hide information in responses.

```console
ui            = true
cluster_addr  = "https://127.0.0.1:8201"
api_addr      = "https://127.0.0.1:8200"
disable_mlock = true

storage "raft" {
  path = "/path/to/raft/data"
  node_id = "raft_node_1"
}

listener "tcp" {
  address             = "127.0.0.1:8200",
  tls_cert_file = "/path/to/full-chain.pem"
  tls_key_file  = "/path/to/private-key.pem"
  redact_addresses    = "true"
  redact_cluster_name = "true"
  redact_version      = "true"
}

telemetry {
  statsite_address = "127.0.0.1:8125"
  disable_hostname = true
}
```

**Example 2.** Result of applying redaction parameters for `API: /sys/health`.

In the `/sys/health/` API call, the `cluster_name` is completely omitted from the response, and version is returned as an empty string (`""`).

```console
$ curl -s https://127.0.0.1:8200/v1/sys/health | jq

{
  "initialized": true,
  "sealed": false,
  "standby": false,
  "performance_standby": false,
  "replication_performance_mode": "disabled",
  "replication_dr_mode": "disabled",
  "server_time_utc": 1715935559,
  "version": "",
  "cluster_id": "be574716-e7e9-a950-ee34-d62d56cd6d4a"
}
```

**Example 3.** Result of applying redaction parameters for `API: /sys/leader`.

In the `/sys/leader/` API call, the `leader_address` and `leader_cluster_address` are set to empty strings (`""`).

```console
$ curl -s https://127.0.0.1:8200/v1/sys/leader | jq

{
  "ha_enabled": true,
  "is_self": true,
  "active_time": "2024-05-13T07:54:20.471072843Z",
  "leader_address": "",
  "leader_cluster_address": "",
  "performance_standby": false,
  "performance_standby_last_remote_wal": 0,
  "raft_committed_index": 78,
  "raft_applied_index": 78
}
```

**Example 4.** Result of applying redaction parameters for `API: /sys/seal-status`.

In the `/sys/seal-status/` API call, the `cluster_name`, `build_date`, and version fields are hidden. The `cluster_name` is completely omitted from the response, while `build_date` and version are returned as empty strings (`""`).

```console
$ curl -s https://127.0.0.1:8200/v1/sys/seal-status | jq

{
  "type": "shamir",
  "initialized": true,
  "sealed": false,
  "t": 3,
  "n": 6,
  "progress": 0,
  "nonce": "",
  "version": "",
  "build_date": "",
  "migration": false,
  "cluster_id": "be574716-e7e9-a950-ee34-d62d56cd6d4a",
  "recovery_seal": false,
  "storage_type": "raft"
}
```

**Example 5**. CLI: `stronghold status`

The CLI command `stronghold status` uses endpoints that support data redaction, so the output hides `Version`, `Build Date`, `HA Cluster`, and `Active Node Address`.

`Version`, `Build Date`, and `HA Cluster` show `n/a` because the corresponding endpoint returned an empty string. On the other hand, `Active Node Address` is shown as `<none>` because the address was omitted in the API response.

```console
stronghold status

Key                     Value
---                     -----
Seal Type               shamir
Initialized             true
Sealed                  false
Total Shares            6
Threshold               3
Version                 n/a
Build Date              n/a
Storage Type            raft
HA Enabled              true
HA Cluster              n/a
HA Mode                 active
Active Since            2024-05-13T07:54:20.471072843Z
Active Node Address     <none>
Raft Committed Index    78
Raft Applied Index      78
```

### Unix

The Unix listener configuration configures Stronghold to listen on the specified Unix domain socket:

```console
listener "unix" {
  address = "/run/vault.sock"
}
```

You can specify the `listener` section multiple times to have Stronghold listen on multiple sockets.

#### Unix listener parameters

* `address` — required string parameter. Specifies the address to bind the Unix socket.

* `socket_mode` — optional string parameter. Changes the permissions and special flags of the Unix socket.

* `socket_user` — optional string parameter. Changes the user owner of the Unix socket.

* `socket_group` — optional string parameter. Changes the group owner of the Unix socket.

#### Example Unix listener configuration

**Example 1.** Listening on multiple sockets.

In this example, Stronghold is configured to listen on the specified socket and the default socket.

```console
listener "unix" {}

listener "unix" {
  address = "/var/run/vault.sock"
}
```

**Example 2.** Listening on multiple interfaces.

In this example, Stronghold is configured to listen on the specified Unix socket, as well as on the loopback interface.

```console
listener "unix" {
  address = "/var/run/vault.sock"
}

listener "tcp" {
  address = "127.0.0.1:8200"
}
```

**Example 3.** Configuring permissions.

This example shows the configuration of permissions and ownership — user and group.

```console
listener "unix" {
  address = "/var/run/vault.sock"
  socket_mode = "644"
  socket_user = "1000"
  socket_group = "1000"
}
```

## Storage section {#storage}

The `storage` section configures the storage backend used for persistent data storage in Stronghold.

Each backend has its own advantages and disadvantages, with trade-offs. For example, some backends provide more reliable backup and restore processes, while others support high availability.

### Configuration

The storage backend is configured in the Stronghold configuration file through the `storage` section.

```console
storage [NAME] {
  [PARAMETERS...]
}
```

For example:

```console
storage "file" {
  path = "/mnt/vault/data"
}
```

For configuration parameters that also read environment variables, the environment variable will take precedence over the values in the configuration file.

### Filesystem storage backend

The filesystem storage backend stores Stronghold data in the file system using a standard directory structure. It can be used in scenarios with a single persistent server or for local development where durability is not critical.

The filesystem storage backend does not support high availability.

Example configuration of the `storage` section with this storage backend:

```console
storage "file" {
  path = "/mnt/vault/data"
}
```

Although Stronghold data is encrypted at rest, appropriate measures must be taken to secure access to the file system.

#### Parameters

* `path` — required string parameter. The absolute path to the directory where data will be stored. If the directory does not exist, Stronghold will create it.

### Raft storage Backend

The integrated storage backend (Raft) is used to store Stronghold data. Unlike other storage backends, it does not operate with a single data source; instead, all nodes in the Stronghold cluster will have a replicated copy of the data. Data is replicated across all nodes using the Raft consensus algorithm.

The Raft storage backend supports high availability.

Example configuration of the `storage` section with a Raft storage backend:

```console
storage "raft" {
  path = "/path/to/raft/data"
  node_id = "raft_node_1"
}
cluster_addr = "http://127.0.0.1:8201"
```

When using the integrated storage backend, you must specify the `cluster_addr` parameter, which represents the address and port used for communication between nodes in the Raft cluster.

Additionally, when using the integrated storage backend, you cannot declare a separate `ha_storage` backend. It is also strongly recommended to set `disable_mlock` to `true` and disable swapping on the system.

#### Parameters

* `path` — string parameter. Specifies the path to the directory in the file system where Stronghold data is stored. This value can be overridden by setting the `VAULT_RAFT_PATH` environment variable.

* `node_id` — string parameter. The node identifier in the Raft cluster. This value can be overridden by setting the `VAULT_RAFT_NODE_ID` environment variable.

* `performance_multiplier` — integer parameter. A multiplier used by servers to scale key Raft timing parameters.

  The `performance_multiplier` setting affects the time Stronghold needs to detect leader failures and elect a leader. This is achieved by requiring more network and computational resources to improve performance. If the value is not set or set to `0`, the default time described below will be used. Lower values tighten the time and increase sensitivity, while higher values loosen the time and reduce sensitivity.

  By default, Stronghold uses lower-performance timing parameters, suitable for servers meeting the minimum Stronghold requirements. Currently, this is equivalent to setting the value to `5`, but the standard may change in future versions of Stronghold if the minimum server profile changes. Setting `performance_multiplier` to `1` configures Raft for the highest performance mode, which is recommended for Stronghold servers in production environments. The maximum allowed value is `10`.

* `trailing_logs` — integer parameter. Controls the number of log entries that remain in the log storage on disk after a snapshot is created. This parameter should only be adjusted if followers cannot catch up with the leader due to a very large snapshot size and high write throughput, which causes log truncation before the snapshot can be fully applied.

  If you need to use this for cluster recovery, consider lowering the write throughput or the volume of data stored in Stronghold. The default value is `10000`, which is suitable for most normal workloads. The `trailing_logs` metric is not equivalent to the `max_trailing_logs` parameter.

* `snapshot_threshold` — integer parameter. Controls the minimum number of Raft log entries between snapshots that are saved to disk. Typically, this low-level parameter does not require modification. In highly loaded clusters with excessive disk I/O, increasing the value can reduce disk load and minimize the likelihood of simultaneous snapshot creation on all servers.

  Increasing the `snapshot_threshold` parameter is a trade-off between disk I/O and disk space, as the Raft log will grow significantly, and the space in the `raft.db` file will not be cleared until the next snapshot. Also, during failure recovery or failover, servers will require more time, as they will need to replay more logs. The default value is `8192`.

* `snapshot_interval` — integer parameter. The interval between snapshots in seconds, controlling how often Raft checks whether a snapshot is needed. To avoid the entire cluster creating a snapshot at the same time, Raft randomly selects a time between the specified interval and its double. By default, this is set to 120 seconds.

* `retry_join` — list parameter. Allows configuring a set of connection parameters for other nodes in the cluster. This set helps nodes find the leader to join the cluster. Multiple `retry_join` sections can be defined.

  If the connection parameters for all nodes in the cluster are known in advance, these sections can be enabled. In this case, once one node is initialized as the leader, the others will use their `retry_join` configuration to find the leader and join the cluster. Note that when using Shamir Secret Sharing, the joined nodes will need to be manually unlocked.

  For more information on the `retry_join` section parameters, see [retry_join section parameters](#parameters-for-retry_join-section).

* `retry_join_as_non_voter` — boolean parameter. When set to `true`, any `retry_join` configuration will join the Raft cluster as a non-voting member. The node will not participate in the Raft quorum but will receive the data replication stream. This allows the cluster to scale reading.

  This option has the same effect as the `-non-voter` flag for the `stronghold operator raft join` command but only affects the voting status when joining via the `retry_join` configuration. The parameter can be overridden by setting the `VAULT_RAFT_RETRY_JOIN_AS_NON_VOTER` environment variable to any non-empty value. This only applies if at least one `retry_join` section is specified.

* `max_entry_size` — integer parameter. Configures the maximum number of bytes for a Raft entry. This applies to both put operations and transactions. Any put operation or transaction that exceeds this configuration value will cause the corresponding operation to fail. Raft has a recommended maximum data size for an entry in the log, based on the current architecture, standard timing, etc.

  The integrated storage also uses a block size — a threshold applied to split large values into blocks. By default, the block size equals the maximum size of a Raft log entry. The default value is `1048576`.

* `autopilot_reconcile_interval` — string parameter. Specifies the time interval after which the autopilot will detect any state changes.

  State changes can indicate various things, such as:
  1. A node, initially added as a non-voting node in the Raft cluster, has successfully completed the stabilization period, qualifying it to be promoted to voting status.
  2. A node should be marked as `unhealthy` in the state API.
  3. A node has been marked as `dead` and should be removed from the Raft configuration.

  The value is specified with a time suffix, for example, `"40s"` (40 seconds) or `"1h"` (1 hour).

* `autopilot_update_interval` — string parameter. Specifies the time interval after which the autopilot will query Stronghold for updates on the relevant information. This includes data such as autopilot configuration and current state, Raft configuration, known servers, the latest Raft index, and statistics for all known servers. The retrieved information will be used to calculate the autopilot's next state. The value is specified with a time suffix, for example, `"40s"` (40 seconds) or `"1h"` (1 hour).

#### Parameters for `retry_join` section

* `leader_api_addr` — string parameter. The IP address of a potential leader node.

* `leader_tls_servername` — string parameter. The TLS server name used when connecting via HTTPS. It must match one of the names in the DNS SANs (Subject Alternative Names) of the leader's TLS certificate. The node uses `leader_tls_servername` to verify the leader’s certificate when attempting to connect to the leader of the cluster. This ensures secure connections and verifies that the connection is made with the correct server.

* `leader_ca_cert_file` — string parameter. Path to the CA certificate file of the potential leader node.

* `leader_client_cert_file` — string parameter. Path to the client certificate file used for authentication when connecting to the leader via TLS. The Raft node presents this certificate when establishing a secure connection with the leader to authenticate itself.

* `leader_client_key_file` — string parameter. Path to the private key file used in conjunction with the client certificate for authentication when connecting to the leader via TLS.

* `leader_ca_cert` — string parameter. The CA certificate value of the potential leader node.

* `leader_client_cert` — string parameter. The client certificate value used for authentication when connecting to the leader via TLS. The Raft node presents this certificate when establishing a secure connection with the leader to authenticate itself.

* `leader_client_key` — string parameter. The private key value used in conjunction with the client certificate for authentication when connecting to the leader via TLS.

Each `retry_join` block can provide TLS certificates either through file paths or as certificate values, but not a combination of both. If a certificate value is provided, it must be specified in a single line using `\n` to denote required line breaks.

## UI section

Stronghold provides a user interface (web interface) for operators. It enables easy creation, reading, updating, and deletion of secrets, authentication, storage unsealing, and much more.

### Enabling UI

By default, the Stronghold user interface is not enabled. To enable it, set the `ui` parameter in the Stronghold server configuration to `true`.

```console
ui = true

listener "tcp" {
  # ...
}
```

### Accessing Stronghold UI

The user interface runs on the same port as the Stronghold listener. To access the UI, at least one `listener` section must be configured.

```console
listener "tcp" {
  address = "10.0.1.35:8200"
  ...
}
```

The user interface in the example provided is accessible via the URL [https://10.0.1.35:8200/ui/](https://10.0.1.35:8200/ui/) from any machine in the subnet — assuming there are no firewalls or the firewalls are properly configured. The UI is also accessible via any DNS entry that resolves to this IP address.

## user_lockout section {#user_lockout}

The `user_lockout` section defines the settings for locking users after unsuccessful login attempts to Stronghold. These settings can be applied globally — for all authentication methods (userpass, ldap, and approle) using the common section name `user_lockout "all"`, or individually for specific methods by specifying their name in the section. Supported values are: `all`, `userpass`, `ldap`, and `approle`.

Configurations specified for a specific authentication method take precedence over settings for all authentication methods using the `user_lockout "all"` section. If both configurations are present, the parameters for the specific method will be applied.

### Parameters for the `user_lockout` section

* `lockout_threshold` — string parameter. Specifies the number of failed login attempts after which the user will be locked out.

* `lockout_duration` — string parameter. Specifies the duration of the user's lockout. The value is specified with a time suffix, for example, `"40s"` (40 seconds) or `"1h"` (1 hour).

* `lockout_counter_reset` — string parameter. Defines the time interval after which the lockout counter is reset if no failed login attempts occur. The value is specified with a time suffix, for example, `"40s"` (40 seconds) or `"1h"` (1 hour).

* `disable_lockout` — boolean parameter. Disables the user lockout feature if set to `true`.

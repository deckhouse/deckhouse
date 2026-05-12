This image provides `/usr/bin/sandbox` for isolated `nginx -t` execution in the `validationIsolationMode: IsolatedProcess` mode of ingress-nginx `1.14` and `1.15`.

`/usr/bin/sandbox` uses the ptrace-based `go-sandbox` runner. Full mode requires the host to allow ptrace-based sandboxing, which in practice means `kernel.yama.ptrace_scope=0` or `1`.

For `IsolatedProcess`, the controller runs:

```text
/usr/bin/sandbox --isolated-process -- /usr/local/nginx/sbin/nginx -c ... -t -e /dev/null
```

In this mode, `sandbox` first creates an exec boundary and starts itself again in `--isolated-process-child` mode with:
- private `user` and `network` namespaces;
- uid/gid mappings for the current non-root user;
- ambient `CAP_NET_ADMIN`, `CAP_NET_BIND_SERVICE`, and `CAP_SYS_CHROOT`.

The child mode then:
- brings `lo` up inside the private network namespace;
- enters `/validation-chroot`;
- launches the usual ptrace/seccomp sandbox path for `nginx -t`.

`/validation-chroot` is prepared in controller images as a hardlinked copy of `/chroot`. The `nginx` binary inside `/validation-chroot/usr/local/nginx/sbin/nginx` is then replaced with a separate uncapped copy so ambient capabilities survive the final `execve()` during isolated validation.

`SANDBOX_DEBUG=true` enables sandbox tracing, and `SANDBOX_DEBUG_CRASH_ON_DENY=true` additionally converts any deny into immediate sandbox termination.

This image provides `/usr/bin/sandbox` for isolated `nginx -t` execution in the `validationIsolationMode: IsolatedProcess` mode of ingress-nginx `1.14` and `1.15`.

`/usr/bin/sandbox` uses the ptrace-based `go-sandbox` runner. Full mode requires the host to allow ptrace-based sandboxing, which in practice means `kernel.yama.ptrace_scope=0` or `1`.

For `IsolatedProcess`, the controller runs:

```text
/usr/bin/sandbox --isolated-process -- /path/to/nginx.conf
```

In this mode, `sandbox` first creates an exec boundary and starts itself again in `--isolated-process-child` mode with:
- private `user` and `network` namespaces;
- uid/gid mappings for the current non-root user;
- ambient `CAP_NET_BIND_SERVICE` and `CAP_SYS_CHROOT`.

The child mode then:
- enters `/validation-chroot`;
- drops CAP_SYS_CHROOT
- launches the usual ptrace/seccomp sandbox path for `nginx -t`.

`/validation-chroot` is prepared in controller images as a hardlinked copy of `/chroot`. 

`SANDBOX_DEBUG=true` enables verbose syscall tracing. For unknown syscalls (for example `getpgid`), debug mode **soft-bans** the call (returns `EACCES` to the traced process) and logs `deny syscall="…" (soft-ban)`. Without debug, the same policy **kills** the traced process and logs `deny syscall="…" (kill)`, which surfaces as `Disallowed Syscall`. Use `SANDBOX_DEBUG_CRASH_ON_DENY=true` to force kill on any file/syscall deny while debugging.

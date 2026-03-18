This image provides `/usr/bin/sandbox` for isolated `nginx -t` execution in the `validationSandboxMode: full` mode of ingress-nginx `1.14`.

`/usr/bin/sandbox` uses the ptrace-based `go-sandbox` runner. Full mode requires the host to allow ptrace-based sandboxing, which in practice means `kernel.yama.ptrace_scope=0` or `1`.

`SANDBOX_DEBUG=true` enables sandbox tracing, and `SANDBOX_DEBUG_CRASH_ON_DENY=true` additionally converts any deny into immediate sandbox termination.

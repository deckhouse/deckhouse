This image provides `/usr/bin/sandbox` for isolated `nginx -t` execution in the `validationIsolationMode: IsolatedProcess` mode of ingress-nginx `1.14` and `1.15`.

`/usr/bin/sandbox` uses the ptrace-based `go-sandbox` runner. Full mode requires the host to allow ptrace-based sandboxing, which in practice means `kernel.yama.ptrace_scope=0` or `1`.

`SANDBOX_DEBUG=true` enables sandbox tracing, and `SANDBOX_DEBUG_CRASH_ON_DENY=true` additionally converts any deny into immediate sandbox termination.

`--allow-dns-to <ip>:<port>` is an opt-in sandbox argument for DNS resolution during isolated validation. When set, the sandbox allows DNS traffic only to the specified endpoint: UDP `sendto()` plus reply reads on the same socket, and TCP fallback via `connect()`/`sendmsg()`/`recvmsg()` to that same `ip:port`. It also permits libc resolution probes that use `connect()` to port `65535` (including musl's loopback `AI_ADDRCONFIG` probes to `127.0.0.1:65535` and `::1:65535`), without treating those sockets as approved for data exchange.

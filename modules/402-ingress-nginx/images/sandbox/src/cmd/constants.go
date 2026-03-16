/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"slices"
	"time"

	"github.com/criyle/go-sandbox/runner"
)

const (
	// Validation with modsecurity + owasp can exceed tiny defaults even on healthy configs.
	sandboxCPUTimeLimit  = 20 * time.Second
	sandboxWallTimeLimit = 25 * time.Second
	sandboxMemoryLimit   = runner.Size(384 << 20) // 384 MiB
)

var sandboxExtraAllowSyscalls = []string{
	// Required by dynamically linked binaries and util-linux tools during startup.
	"set_tid_address",
	"set_robust_list",
	"futex",
	"rseq",
	"getpid",
	"gettid",
	"prlimit64",
	"getrandom",
	"getuid",
	"getgid",
	"geteuid",
	"getegid",
	"getppid",
	"uname",
	"wait4",
	"poll",
	// Required for additional features (geoip,opentelemetry,etc...).
	"membarrier",
	"eventfd2",
	"pipe",
	"pipe2",
	// Required by `unshare -S ... -R /chroot ...` in nginx wrapper.
	"unshare",
	"chroot",
	"chdir",
	"setgroups",
	"setuid",
	"setgid",
	"setresuid",
	"setresgid",
	// Required by nginx master initialization during `nginx -t`.
	"sched_getaffinity",
	"epoll_create1",
	"mkdir",
	"chown",
	// Required by nginx when validating listening sockets during `nginx -t`.
	"socket",
	"bind",
	"listen",
	"setsockopt",
	"statfs",
}

var sandboxExtraReadBase = []string{
	"/etc/nginx/",
	"/etc/ld-musl-x86_64.path",
	"/usr/ssl/openssl.cnf",
	"/etc/passwd",
	"/etc/group",
	"/etc/ingress-controller/ssl/",
	"/etc/ingress-controller/auth/",
	"/etc/ingress-controller/geoip/",
	"/etc/ingress-controller/telemetry/",
	"/bin/sh", // for modsecurity
	"/lib/",
	"/usr/lib/",
	"/usr/local/lib/",
	"/usr/local/modsecurity/lib/",
	"/modules_mount/etc/nginx/modules/otel/",
	"/chroot/*", // allow only top-level files in /chroot (not recursive into /chroot/<dir>/...) for modsecurity "/chroot/unicode.mapping", "/chroot/scanners-user-agents.data" etc ...
	"/usr/local/nginx/sbin/nginx",
}

var sandboxExtraWrite = []string{
	"/dev/null",
	"/dev/tty", // for modsecurity
	"/tmp/nginx/",
	"/var/log/modsec_audit.log",
	"/var/log/audit/",
	"/proc/self/uid_map",
	"/proc/self/gid_map",
	"/proc/self/setgroups",
}

func getSandboxExtraRead(realPathNginxConf string) []string {
	extraRead := make([]string, 0, len(sandboxExtraReadBase)+1)
	extraRead = append(extraRead, sandboxExtraReadBase...)
	extraRead = append(extraRead, realPathNginxConf)

	return extraRead
}

func getSandboxExtraWrite() []string {
	return slices.Clone(sandboxExtraWrite)
}

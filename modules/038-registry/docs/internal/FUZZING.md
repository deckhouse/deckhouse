# Fuzzing the `registry` module

Native Go fuzz harnesses for the `038-registry` module, derived from the module's
threat model (`registry-threat-model.md`, sections 5 and 6).

The harnesses live next to the code they exercise, so they span two Go modules:

| Package | File |
| --- | --- |
| `go_lib/registry/models/node-services` | `fuzz_test.go` |
| `go_lib/registry/models/bashible` | `fuzz_test.go` |
| `go_lib/registry/models/moduleconfig` | `fuzz_test.go` |
| `go_lib/registry/pki` | `fuzz_test.go` |
| `go_lib/registry/helpers` | `fuzz_test.go` |
| `go_lib/registry/const` | `fuzz_test.go` |
| `modules/038-registry/hooks/orchestrator` | `params_fuzz_test.go` |
| `modules/038-registry/hooks/orchestrator/bashible` | `fuzz_test.go` |
| `modules/038-registry/images/nodeservices-manager/app/internal/staticpod` | `fuzz_test.go` |
| `modules/038-registry/images/mirrorer/app/internal/config` | `fuzz_test.go` |
| `modules/038-registry/images/syncer/app/pkg/config` | `fuzz_test.go` |
| `modules/038-registry/images/registry-proxy/reloader/src` | `fuzz_test.go`, `watcher_fuzz_test.go` |

## Coverage against section 6 of the threat model

Section 6 of `registry-threat-model.md` splits the fuzzing requirement into ten
harnesses. This is what each one asks for and what exists.

| # | Required | State |
| --- | --- | --- |
| 1 | Node configuration: `ProxyConfig` (`http`, `https`, `no_proxy`) and `LocalMode.Upstreams`; parsing and validation of `go_lib/registry/models/node-services`, plus static pod manifest and service config generation in `internal/staticpod` | **Covered** |
| 2 | Upstream registry parameters (address, scheme, path, CA) — at the input of `hooks/orchestrator/*` **and** at the output, in the generated `distribution` and `mirrorer` configs | **Covered** |
| 3 | `distribution` protocol and storage: structural fuzzing of manifests and layer descriptors, plus a separate stateful HTTP harness for the blob upload tract | **Covered** (fork) |
| 4 | `auth` service: `/auth` with `service`, `scope`, `account`, repeated and contradictory scopes, bad encoding, anomalous length, `Authorization` header; ACL invariants | **Covered** (fork) |
| 5 | Fork-specific `distribution` code: `registry/proxy_headers.go`, `registry/auth_proxy.go`, the reworked `registry/proxy` package | **Covered** (fork) |
| 6 | `mirrorer`: mirroring configuration (`Upstreams`, accounts, replica addresses) | **Covered** |
| 7 | `syncer` — explicitly optional, outside the threat perimeter (TM-16 assessed as not applicable) | **Covered** |
| 8 | NGINX in the `stream` context: the client direction (data arriving at `127.0.0.1:5001`) and the upstream direction (data from the registry) as **separate** harnesses — malformed and truncated streams, anomalous sizes and transfer rates | **Covered** (fork) |
| 9 | `registry-proxy-reloader`: the input `nginx_new.conf` for `watcher.go` (partially written, truncated, syntactically invalid) **and** the `file_comparison.go` functions | **Covered** |
| 10 | `proxyEndpoints`: the source `Node.status.addresses[type=InternalIP]`, the final elements, and three code sites — `hooks/orchestrator/bashible`, `go_lib/registry/const`, `go_lib/registry/models/bashible` | **Covered** |

### Harness 8, how it is built

The nginx `stream` context has no protocol parser in this build: the module
builds with `--with-stream` and without `stream_ssl_module` or
`stream_ssl_preread_module`, so the balancer copies bytes without looking at
them. What section 6 and TM-25 ask for is not a parser but robustness of **TCP
connection handling** -- truncated streams, anomalous sizes and transfer rates,
behaviour at connection and descriptor exhaustion, absence of leaks -- and none
of that needs a parser to be meaningful.

`fuzz/stream_fuzz.c` in the nginx fork reads each input as a *script* over
connections, in two modes, because the source of untrusted data differs:

- **client** — the driver is the client. It connects to the listener and plays
  writes, bursts, delays, half-closes and resets across up to sixteen
  simultaneous connections, with a well-behaved upstream in a thread.
- **upstream** — the driver is the registry. A well-formed client request goes
  through nginx while the upstream answers badly: truncated, endless, one byte
  at a time, refused, reset, silent, or a single large burst.

The subject is the real nginx binary in another process, so `fuzz/stream-run.sh`
is what makes it a test. It starts nginx under AddressSanitizer with the
configuration the bashible step renders, points the driver at it, and afterwards
checks three things: nginx is still alive, its descriptor count has not grown,
and its log holds no sanitizer report. The shutdown is a SIGQUIT rather than a
kill, which is what makes the last check mean anything -- nginx frees its pools
on a graceful exit, so a LeakSanitizer report after one is a real leak, while
after a SIGKILL every pool still held would be reported and the check would be
worthless.

The configuration-parser harness stays as an addition rather than a substitute:
it covers the path from `registry.proxyEndpoints` through the generated file to
`nginx -t`, which is a real path from cluster state, and it is what found the
`add_code` defect.

### What closing 2, 6, 7, 9 and 10 turned up

Every one of those harnesses reproduced something. The fixes are in the same
change.

- **The producer of the bashible configuration did not validate what it wrote**
  (harness 10, TM-17 and TM-18, both critical). `hooks/orchestrator/bashible`
  declares `type Config bashible.Config`, a distinct named type that carries no
  `Validate` method, so the model's rules were never applied on the write path.
  A node address of `$(id > /tmp/pwned)` produced
  `proxyEndpoints = ["$(id > /tmp/pwned):5001"]` in the secret. The consumer,
  `bashible-apiserver`, does validate -- twice, in `controller.go` -- so the value
  never reached `server <value>;`; what it did was make the apiserver refuse the
  secret, stopping registry configuration updates for the whole cluster over one
  malformed node address, with the reason visible only on the other side.
  `build()` now validates before returning.
- **`imagesRepo` admitted a path traversal and impossible ports** (harness 2,
  TM-06 critical). `imagesRepoRegexp` allows dots in the host and treats `/` as a
  separator, so `../../../etc/cron.d/x` matched and split into the host `..`,
  which becomes a directory name under `/etc/containerd/registry.d` and a table
  key in `hosts.toml`; and the port was only counted, not ranged, so `:0` and
  `:99999` passed. `helpers.RegistryAddress` now runs alongside the pattern, and
  `helpers.URLPath` rejects a `.` or `..` segment.
- **Nested validation was silently disabled in two config models** (harnesses 6
  and 7). `mirrorer`'s `Users`/`UserInfo` and `syncer`'s `Registry`/`User`
  declared `Validate()` on pointer receivers while appearing as struct fields by
  value, so ozzo never found the `Validatable` interface and none of the rules
  inside them ran -- `address` being required included. Both are value receivers
  now, with the `var _ validation.Validatable` assertions that would have caught
  it. On top of that both models now check that an address is one
  `name.NewRegistry` can construct, that credentials carry no CR, LF or NUL
  before going into an `Authorization` header, and that `parallelizm` is not
  negative, which `errgroup.SetLimit` reads as no limit at all.
- **The reloader signalled the first process that merely mentioned nginx**
  (harness 9, TM-21). `sendReloadSignal` matched `strings.Contains(cmdline,
  "nginx")` and stopped at the first hit, which also matches a worker -- where
  SIGHUP means graceful shutdown -- the `nginx -t` child the reloader spawns
  itself, and anything like `tail -f /var/log/nginx/error.log`. Because the copy
  happens before the signal, a wrong hit leaves the files equal and every later
  event skips the reload too: applied on disk, never loaded. It works in the
  shipped container only because the master is PID 1. `isNginxMasterCmdline`
  now identifies the master, and `TestNginxMasterSelection` pins the cases.

## Coverage against the threat model

| Threat | Harness | Invariant |
| --- | --- | --- |
| TM-02 (arbitrary static pod on a control plane node via `ProxyConfig`) | `FuzzStaticPodManifestProxyEnvs`, `FuzzProxyConfigValidate` | The rendered manifest keeps its structure and reproduces the proxy values verbatim; `ProxyConfig.Validate()` only accepts values that can be rendered safely |
| TM-08 (injection into `distribution` / `mirrorer` configuration) | `FuzzDistributionConfigUpstream`, `FuzzMirrorerConfigUpstreams`, `FuzzUpstreamRegistryValidate`, `FuzzLocalModeUpstreams` | `remoteurl`, `local` and the `remote` list keep their intended values; the upstream registry and mirror targets are constrained before rendering |
| TM-17 (NGINX directive and shell injection through `proxyEndpoints`) | `FuzzContextProxyEndpoints`, `FuzzGenerateProxyEndpoints`, `FuzzConfigBuilderMasterNodesIPs` | Anything `Validate()` accepts is an `<ip>:<port>` pair, free of NGINX and shell metacharacters; the producer never turns an unacceptable value into an acceptable one; the configuration the hook writes satisfies the model's rules |
| TM-18 (redirecting image pulls through `hosts` / mirrors) | `FuzzContextHosts`, `FuzzConfigBuilderUnmanaged` | `hosts` keys are safe directory names, mirror hosts are shell-safe and the scheme is `http` or `https` |
| TM-06 (substituting the upstream registry when proxying) | `FuzzRegistrySettingsValidate`, `FuzzDeckhouseSettingsJSON`, `FuzzParamsStateRoundTrip` | An accepted `imagesRepo` splits into a host and a path safe for every sink; exactly one mode section is populated; persisted state reads back unchanged |
| TM-01, TM-07 (module PKI) | `FuzzDecodeCertificate`, `FuzzDecodePrivateKey`, `FuzzDecodeCertKey`, `FuzzValidateCertWithCAChain` | Decoding never panics; a certificate only verifies against the CA that issued it |
| TM-15 (storage ACL) | `FuzzAuthConfigUsers` | The `auth` config keeps five ACL entries, every entry keeps its `account` constraint and the read-only account never gains write actions |
| TM-21, TM-22, TM-23 (applying the balancer configuration) | `FuzzFileContentsEqual`, `FuzzCopyFile`, `FuzzNginxReload`, `TestNginxMasterSelection` | Equality is reported exactly when the bytes match, never for a missing or truncated file; the bytes applied are the bytes that passed the check; SIGHUP goes to the master process |
| TM-05, TM-09, TM-13 (credential handling in the supply path) | `FuzzCredsFromDockerCfg`, `FuzzDockerCfgRoundTrip`, `FuzzSplitAddressAndPath` | Credentials survive the encode/decode pair unchanged; the registry host never carries a path separator |
| TM-16 (image transfer by `syncer`, outside the threat perimeter) | `FuzzSyncerConfig` | An accepted address is one the registry client can construct; credentials are carriable in an HTTP header |
| Configuration robustness | `FuzzConfigJSONValidate`, `FuzzConfigJSONRoundTrip`, `FuzzParseConfig` | An arbitrary secret payload never panics the controller or the hooks, and validation is deterministic |

Harnesses 3, 4, 5 and 8 of the threat model's plan need the third-party binaries
under test in-process, so they live in the forks rather than here; see
"Third-party forks" below and the section-6 matrix above for their state.

## Running

Seed corpora run as ordinary unit tests:

```shell
cd go_lib/registry && go test ./...
go test ./modules/038-registry/hooks/orchestrator/...
cd modules/038-registry/images/nodeservices-manager/app && go test ./...
cd modules/038-registry/images/mirrorer/app && go test ./...
cd modules/038-registry/images/syncer/app && go test ./...
cd modules/038-registry/images/registry-proxy/reloader && go test ./...
```

Actual fuzzing takes one target at a time:

```shell
cd go_lib/registry
go test ./models/bashible/ -run FuzzContextProxyEndpoints -fuzz '^FuzzContextProxyEndpoints$' -fuzztime 60s
```

Inputs that fail are written to `testdata/fuzz/<Target>/` and are replayed by
`go test` from then on, so a finding stays a regression test. Reproduce a single
one with:

```shell
go test ./helpers/ -run 'FuzzSplitAddressAndPath/3e3b70dba384074d'
```

## Where the invariants are enforced

The harnesses assert properties; two layers uphold them.

**Rendering escapes.** `internal/staticpod/templates.go` exposes two template
functions and every substitution goes through one of them:

- `quote` renders a YAML double-quoted scalar. It is `strconv.Quote` plus a
  rejection of input that is not valid UTF-8: Go writes such bytes as `\xNN`
  while YAML reads `\xNN` as the code point `U+00NN`, so the value has no YAML
  representation and is refused rather than silently changed.
- `hostPort` joins a host and a port with `net.JoinHostPort`, which brackets IPv6
  addresses that plain concatenation would leave ambiguous.

**Validation constrains the domain.** `go_lib/registry/helpers/validate.go`
holds the rules the models apply, each documented with the sink it protects:
`RegistryHost`, `RegistryAddress`, `RegistryAccountName`, `IPPort`,
`ProxyEndpoint`, `MirrorHost`, `IPAddress`, `URLScheme`, `URLPath`, `ProxyURL`,
`NoProxyList` and `EncodableString`. They are used by `models/node-services`
(`ProxyConfig`, `LocalMode`, `UpstreamRegistry`, `User`), by `models/moduleconfig`
(`RegistrySettings.ImagesRepo`) and by `models/bashible` (`Context`, `Config`,
mirror hosts and the `hosts` map keys, which ozzo cannot reach because it
validates map values, not keys).

Escaping is not available for every sink. An NGINX `server` directive and a
directory name in a shell command have no quoting of their own, so
`proxyEndpoints` and the `hosts` keys are safe only because validation
constrains them.

The two bashible steps that write those files use an **unquoted** heredoc, and
that is deliberate: in Proxy and Local mode dhctl writes
`helpers.NodeIPPlaceholder` where the node's own address belongs, and the shell
expanding that body is the only thing that resolves it. So the body is subject to
parameter and command substitution as root, and validation -- not quoting -- is
what keeps a `$(...)` out of it. `helpers.ProxyEndpoint` and `helpers.MirrorHost`
admit that one literal and nothing else that carries a shell metacharacter.

Two limits belong to the formats rather than to the code, and the harnesses stay
inside them deliberately, each with the reason recorded at the guard:

- A value that is not valid UTF-8 has no YAML representation. `requireUTF8`
  asserts that the renderer refuses it, then skips.
- YAML limits a simple key to 1024 characters, measured on the rendered key.
  `RegistryAccountName` bounds account names to 255 characters, and the auth
  harness skips names whose quoted form would not fit.

## Third-party forks

Three of the module's images are built from forks, and the code that reads
attacker-shaped input lives in them rather than here. Their harnesses live on a
`deckhouse-fuzzing` branch in each fork, next to the code they test.

| Fork | Branch | Harnesses |
| --- | --- | --- |
| `3p-distribution` | `deckhouse-fuzzing` | `FuzzUnmarshalManifest`, `FuzzProxyHeadersClientCert`, `FuzzBlobUploadSession`, `FuzzManifestPut`, `FuzzAuthProxyRequest`, `FuzzProxyCachePoisoning`, `TestManifestGetAcceptIsCaseInsensitive` |
| `3p-docker_auth` | `deckhouse-fuzzing` | `FuzzAuthEndpoint`, `FuzzAuthRequest`, `TestStaticUserWithoutPasswordAuthenticatesAnyPassword`, `TestScopeTypeIsNotAnchored`, `TestAccountOverridesAreRefused` |
| `nginx` | `deckhouse-fuzzing` (from `release-1.27.3`) | `fuzz/conf_parse_fuzzer.c`, `fuzz/stream_fuzz.c`, see `fuzz/README.md` |

### What they cover

`FuzzUnmarshalManifest` drives `distribution.UnmarshalManifest` over all four
manifest media types and asserts the content-addressing invariants: the
descriptor's digest and size describe exactly the bytes received, `Payload()`
returns those bytes, and a re-parse is identical.

`FuzzBlobUploadSession` and `FuzzManifestPut` drive the real HTTP handlers.
The upload harness reads a fuzzed byte string as a program over an upload
session -- open, append, append out of order, finalise, resume a deleted
session -- so it reaches orderings the protocol does not describe. Both assert
that no client-shaped request produces a 5xx, that a blob only exists under a
digest describing its bytes, and that an accepted manifest names only blobs the
registry already holds.

`FuzzProxyHeadersClientCert` covers the fork's own `real_ip` filter (TM-10 /
AS-10) and is the one that reproduces a defect. See below.

`FuzzAuthProxyRequest` covers `registry/auth_proxy.go`, the reverse proxy the
fork puts in front of the authentication service, against an upstream that
answers badly. It asserts two things about what the auth service is told: the
outbound request reaches the configured URL's path and no other, so an inbound
path cannot steer it elsewhere in the auth service; and the client address the
auth service will act on is the real one. The second spans the two forks --
`docker_auth` resolves the client address from the header named by
`real_ip_header`, which the module sets to `X-Forwarded-For`, and takes the
element at `real_ip_pos`, default 0, and its ACL can match on `ip`. The harness
confirms rather than assumes that `SetXForwarded` replaces an inbound chain
instead of appending to it, so a client cannot choose that value.

`FuzzProxyCachePoisoning` covers the reworked `registry/proxy` package -- the
pull-through cache every node pulls from in Proxy mode, filled by a registry
outside the cluster (TM-06, critical). The upstream is hostile in the two ways it
can be, independently: the descriptor it reports from `Stat` and the bytes it
serves from `Open`. The invariant is the one a content-addressed cache cannot
give up -- whatever ends up in the local store under a digest must hash to that
digest -- and it matters more than a single bad response, because a poisoned
entry is served to every later client without the upstream being consulted again.

`FuzzAuthRequest` completes harness 4 with the three inputs `FuzzAuthEndpoint`
holds fixed: `account` as a free-form parameter including the case where it
disagrees with the authenticated user, the raw `Authorization` header, and
repeated `scope` query parameters -- `ParseRequest` iterates `req.Form["scope"]`,
so `?scope=a&scope=b` is a different path from `?scope=a b`.

`FuzzAuthEndpoint` drives the `/auth` endpoint against both ACL policies the
module ships -- the node-side one from
`images/nodeservices-manager/app/internal/staticpod/templates/auth/config.yaml.tpl`
and the single pull-only rule from `templates/inclusterproxy/secret.yaml`. It
does not reimplement the ACL; it asserts bounds the policies fix: a token never
carries an action the client did not request, never names a resource the request
did not name, the read-only account and the mirror puller never exceed `pull`
(except the puller against `registry:catalog`, which its own entry grants), and
an account with no user entry gets 401 and no token.

The nginx target is the configuration parser, not the data path. The module
builds nginx with `--with-stream` and without `stream_ssl_module` or
`stream_ssl_preread_module`, so the node load balancer parses no protocol at
all -- there is nothing on the data path to fuzz. The configuration parser, by
contrast, reads a file whose `upstream` entries come from
`registry.proxyEndpoints`. `fuzz/README.md` in the fork has the details.

It runs coverage-guided with libFuzzer, AddressSanitizer and
UndefinedBehaviorSanitizer, seeded with the configuration the bashible step
actually generates -- verbatim, plus the one-endpoint, 64-endpoint and
bracketed-IPv6 variants -- and with a dictionary of every directive name in
nginx's source. On `release-1.27.3` the module's own configuration parses fully,
so `ngx_stream_block()` and the `upstream`/`server` handlers are on the covered
path.

### Findings

`FuzzProxyHeadersClientCert` reproduces **AS-10**. `registry/proxy_headers.go`
loops over every element of `r.TLS.PeerCertificates` and trusts
`X-Forwarded-For` if *any* of them verifies against the configured CA. The
listener is configured with `tls.RequestClientCert`, so TLS proves possession of
the private key for exactly one certificate -- the leaf. Every other element is
bytes the peer chose to attach. A peer holding no CA-issued key therefore claims
any source address by attaching the public part of any certificate the Ingress
CA ever issued, which is visible to anyone who has observed a connection from
the Ingress controller. The configured `CN` does not help: it is checked against
whichever certificate verified, not against the leaf. The harness also shows
that an *expired* CA-issued certificate as the leaf, or one without
`clientAuth`, is accepted the same way.

Two lower-severity findings in `distribution`, both stated by tests rather than
left to the fuzzer:

- A manifest whose `schemaVersion` is absent or not 2 answers **500** instead of
  400. `verifyManifest` accumulates every other failure into
  `distribution.ErrManifestVerification`, which maps to `MANIFEST_INVALID`, but
  the schema-version branch returns a bare `fmt.Errorf` that falls through to
  `UNKNOWN` (`registry/storage/schema2manifesthandler.go:75`, and the same in
  `ocimanifesthandler.go:69`). A client can produce it with a 15-byte body.
  `FuzzManifestPut` reports it from a seed; set `FUZZ_ALLOW_KNOWN_5XX=1` to fuzz
  past it.
- `Accept` is matched case-sensitively (`registry/handlers/manifests.go:109`)
  while `Content-Type` goes through `mime.ParseMediaType`, which lowercases. A
  media type is case-insensitive, so a client naming schema2 in a different case
  is treated as one that does not understand schema2 at all: the registry
  rewrites the stored manifest to schema1 -- a different document under a
  different digest -- and for any manifest built from a config without history
  that rewrite fails, so a valid stored manifest reads back as 400.
  `TestManifestGetAcceptIsCaseInsensitive` states it.

In `nginx`, the dictionary run found undefined behaviour in the configuration
parser, in code duplicated across both the stream and the http side:

```text
src/stream/ngx_stream_script.c:663  applying non-zero offset N to null pointer
src/http/ngx_http_script.c:800      applying non-zero offset N to null pointer
```

`ngx_stream_script_add_code()` relocates a stored pointer when the push
reallocates the code array, guarded by `if (code)` -- but `code` is the *address*
of the pointer to relocate, so the guard is always true and never tests the
pointer itself. Every caller compiling a complex value passes `&sc->main`, which
`ngx_stream_compile_complex_value()` leaves at `NULL`, so a reallocating push
evaluates `NULL + delta`. `sc->main` is write-only on both paths, so nothing
dereferences the result and there is no reachable crash in this build -- it is
undefined behaviour and a `NULL` that silently becomes non-null. The fix is to
test the pointer instead of its address.

The UB arrives from many directives, not one: `ngx_stream_proxy_pass`,
`ngx_stream_proxy_bind`, `ngx_conf_split_clients_block`,
`ngx_http_set_complex_value_slot`, `ngx_http_set_predicate_slot` and
`ngx_http_fastcgi_merge_loc_conf` all reach it. It is a property of the shared
`add_code` helper, so anything compiling a complex value gets there.

It is **not reachable from the module** -- but not for the reason it first
appears. `proxy_pass` is in the generated configuration and *does* take a complex
value, so "none of the module's directives takes one" would be wrong. What keeps
it out of reach is that the module never puts an nginx variable anywhere in that
file: `ngx_stream_compile_complex_value()` short-circuits a constant and compiles
nothing, so the reallocating push is never reached. The one `$` in the template,
`${discovered_node_ip}`, is resolved by the shell that writes the file and is a
literal IP by the time nginx reads it.

Both halves are checked rather than argued -- `fuzz/findings/proxy_pass-stream.conf`
reproduces it through `proxy_pass "a$u-b"`, and `fuzz/corpus/registry-proxy.conf`,
the generated file itself, stays clean. That second one is worth watching: it
stays clean only while nothing puts an nginx variable into the generated
configuration, and a change that did would make this defect reachable from
cluster state.

The reproducers live in `fuzz/findings/` rather than in the corpus, so a corpus
replay stays a check on the harness.

In `docker_auth`, `FuzzAuthEndpoint` found nothing over 2.1M executions against
both policies. Three behaviours are pinned by tests instead, because they are
upstream by design but remove the margin for error from whatever writes the
config:

- A user entry with no `password` key authenticates **any** password
  (`authn/static_auth.go`). The node template writes `password: {{ quote
  .PasswordHash }}` unconditionally, so an empty hash still denies access; a
  change that made the key conditional would silently create a passwordless
  account.
- `scopeRegex` is not anchored, so the scope type is the first run of lowercase
  alphanumerics anywhere in the field: `Xrepository` and `repository!` both name
  `repository`. Not an escalation under either shipped policy, which decide on
  the account -- but an ACL entry that restricted by type could not assume the
  type is what the client sent.
- `parseScope` never returns the scope class. `scopeRegex` has two capture
  groups, so `FindStringSubmatch` always yields three elements and the branch
  that would return the class is unreachable. Nothing downstream reads it, so it
  is dead code -- but a future ACL wanting to distinguish `repository(plugin)`
  from `repository` would find the field silently empty rather than absent.

### Running them

The forks are separate checkouts, not part of this module's build.

```sh
# distribution
cd <3p-distribution> && git switch deckhouse-fuzzing
go test ./registry/ ./registry/handlers/ ./registry/proxy/ .    # findings and seeds
FUZZ_ALLOW_KNOWN_5XX=1 go test ./registry/handlers/ -fuzz FuzzManifestPut
go test ./registry/proxy/ -fuzz FuzzProxyCachePoisoning
# The auth proxy makes a real HTTP round trip per iteration; bound the workers.
go test ./registry/ -fuzz FuzzAuthProxyRequest -parallel 4

# docker_auth
cd <3p-docker_auth>/auth_server && git switch deckhouse-fuzzing
GOTOOLCHAIN=go1.25.0 go test ./server/ ./authz/
GOTOOLCHAIN=go1.25.0 go test ./server/ -fuzz FuzzAuthEndpoint
GOTOOLCHAIN=go1.25.0 go test ./server/ -fuzz FuzzAuthRequest

# nginx -- needs linux/amd64; see the note below
cd <nginx> && git switch deckhouse-fuzzing
fuzz/build.sh libfuzzer
fuzz/run.sh -runs=0                                  # replay the corpus
fuzz/run.sh -fork=8 -max_total_time=1800 fuzz/corpus-new fuzz/corpus
fuzz/conf_parse_fuzz fuzz/findings/split_clients-stream.conf   # the finding above

# The run stops on the finding above. To survey every distinct UB site in one
# run instead, rebuild the checks as recoverable -- halt_on_error=0 on its own
# does nothing, because the default build compiles them non-recoverable:
NGX_FUZZ_RECOVER=1 fuzz/build.sh libfuzzer
UBSAN_OPTIONS=print_stacktrace=1:halt_on_error=0 fuzz/run.sh -fork=8 -max_total_time=1800

# The two data directions (harness 8). Each brings nginx up under
# AddressSanitizer with the module's own configuration and checks it afterwards.
fuzz/stream-run.sh -runs=2000
fuzz/stream-run.sh client -runs=5000
fuzz/stream-run.sh upstream -runs=5000
```

The nginx target has to run on `linux/amd64`, which is also the platform the
module ships on. Two reasons, both found the hard way on `darwin/arm64`:
AddressSanitizer does not run at all there under Command Line Tools 17 -- even a
trivial `int main(void){return 0;}` hangs at startup, which also hangs nginx's
own `configure` probes -- and Apple's toolchain ships no libFuzzer runtime. On
top of that, `use epoll;` is invalid off Linux, so the module's own generated
configuration aborts at that line and the `stream` block is never reached, which
is most of what there is to cover.

## Writing new harnesses

Two oracles carry most of the value here and are worth reusing:

- **Structural equivalence** (`assertSameShape` in the `staticpod` package).
  Render the template twice — once with the fuzzed value, once with an inert
  placeholder of the same emptiness — and compare the documents' shapes: mapping
  keys and sequence lengths are kept, scalars are reduced to their type. Any
  substitution that adds a key, changes a scalar's type or breaks the document
  fails. This is what distinguishes the escaped templates from the unescaped
  ones: the same oracle passes on `{{ quote .X }}` and fails on `{{ .X }}`.

- **Validation implies sink safety** (`go_lib/registry/models/*`). Take a value
  accepted by `Validate()` and assert it is safe for every sink it reaches. Each
  assertion names the sink in its failure message, so a finding reads as a
  statement about the data path rather than about the fuzzer.

Keep an oracle inside the domain the code can actually receive. Where an
assertion is deliberately narrowed — invalid UTF-8 in `ComputeHash` and in the
docker config round-trip, which `json.Marshal` maps to U+FFFD — the reason is
recorded at the guard.

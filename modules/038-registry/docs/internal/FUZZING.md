# Fuzzing the `registry` module

Native Go fuzz harnesses for the `038-registry` module, derived from the module's
threat model (`registry-threat-model.md`, sections 5 and 6).

The harnesses live next to the code they exercise, so they span two Go modules:

| Package | File |
| --- | --- |
| `go_lib/registry/models/node-services` | `fuzz_test.go` |
| `go_lib/registry/models/bashible` | `fuzz_test.go` |
| `go_lib/registry/pki` | `fuzz_test.go` |
| `go_lib/registry/helpers` | `fuzz_test.go` |
| `modules/038-registry/images/nodeservices-manager/app/internal/staticpod` | `fuzz_test.go` |
| `modules/038-registry/images/registry-proxy/reloader/src` | `fuzz_test.go` |

## Coverage against the threat model

| Threat | Harness | Invariant |
| --- | --- | --- |
| TM-02 (arbitrary static pod on a control plane node via `ProxyConfig`) | `FuzzStaticPodManifestProxyEnvs`, `FuzzProxyConfigValidate` | The rendered manifest keeps its structure and reproduces the proxy values verbatim; `ProxyConfig.Validate()` only accepts values that can be rendered safely |
| TM-08 (injection into `distribution` / `mirrorer` configuration) | `FuzzDistributionConfigUpstream`, `FuzzMirrorerConfigUpstreams`, `FuzzUpstreamRegistryValidate`, `FuzzLocalModeUpstreams` | `remoteurl`, `local` and the `remote` list keep their intended values; the upstream registry and mirror targets are constrained before rendering |
| TM-17 (NGINX directive and shell injection through `proxyEndpoints`) | `FuzzContextProxyEndpoints` | Anything `Context.Validate()` accepts is an `<ip>:<port>` pair, free of NGINX and shell metacharacters |
| TM-18 (redirecting image pulls through `hosts` / mirrors) | `FuzzContextHosts` | `hosts` keys are safe directory names, mirror hosts are shell-safe and the scheme is `http` or `https` |
| TM-01, TM-07 (module PKI) | `FuzzDecodeCertificate`, `FuzzDecodePrivateKey`, `FuzzDecodeCertKey`, `FuzzValidateCertWithCAChain` | Decoding never panics; a certificate only verifies against the CA that issued it |
| TM-15 (storage ACL) | `FuzzAuthConfigUsers` | The `auth` config keeps five ACL entries, every entry keeps its `account` constraint and the read-only account never gains write actions |
| TM-22 (partially written balancer configuration) | `FuzzFileContentsEqual`, `FuzzCopyFile` | Equality is reported exactly when the bytes match, never for a missing or truncated file; the applied copy is byte-identical to the validated one |
| TM-05, TM-09, TM-13 (credential handling in the supply path) | `FuzzCredsFromDockerCfg`, `FuzzDockerCfgRoundTrip`, `FuzzSplitAddressAndPath` | Credentials survive the encode/decode pair unchanged; the registry host never carries a path separator |
| Configuration robustness | `FuzzConfigJSONValidate`, `FuzzConfigJSONRoundTrip` | An arbitrary secret payload never panics the controller or the hooks, and validation is deterministic |

Harnesses 3, 4 and 8 of the threat model's plan — the OCI Distribution protocol,
the `docker_auth` HTTP endpoint and the NGINX `stream` context — are not covered
here. They need the third-party binaries under test in-process (a Go harness
inside the `3p-distribution` / `3p-docker_auth` forks) or a C fuzzer with
sanitizers for NGINX and OpenSSL, and belong in those repositories.

## Running

Seed corpora run as ordinary unit tests:

```shell
cd go_lib/registry && go test ./...
cd modules/038-registry/images/nodeservices-manager/app && go test ./...
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
`RegistryHost`, `RegistryAccountName`, `IPPort`, `IPAddress`, `URLScheme`,
`URLPath`, `ProxyURL`, `NoProxyList` and `EncodableString`. They are used by
`models/node-services` (`ProxyConfig`, `LocalMode`, `UpstreamRegistry`, `User`)
and by `models/bashible` (`Context`, `Config`, mirror hosts and the `hosts` map
keys, which ozzo cannot reach because it validates map values, not keys).

Escaping is not available for every sink. An NGINX `server` directive and a
directory name in a shell command have no quoting of their own, so
`proxyEndpoints` and the `hosts` keys are safe only because validation
constrains them. The two bashible steps that write these files also use a quoted
heredoc (`<< "EOF"`), which removes shell expansion from the body as a second
layer.

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
| `3p-distribution` | `deckhouse-fuzzing` | `FuzzUnmarshalManifest`, `FuzzProxyHeadersClientCert`, `FuzzBlobUploadSession`, `FuzzManifestPut`, `TestManifestGetAcceptIsCaseInsensitive` |
| `3p-docker_auth` | `deckhouse-fuzzing` | `FuzzAuthEndpoint`, `TestStaticUserWithoutPasswordAuthenticatesAnyPassword`, `TestScopeTypeIsNotAnchored`, `TestAccountOverridesAreRefused` |
| `nginx` | `deckhouse-fuzzing` (from `release-1.27.3`) | `fuzz/conf_parse_fuzzer.c`, see `fuzz/README.md` |

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
go test ./registry/ ./registry/handlers/ .                     # findings and seeds
FUZZ_ALLOW_KNOWN_5XX=1 go test ./registry/handlers/ -fuzz FuzzManifestPut

# docker_auth
cd <3p-docker_auth>/auth_server && git switch deckhouse-fuzzing
GOTOOLCHAIN=go1.25.0 go test ./server/ ./authz/
GOTOOLCHAIN=go1.25.0 go test ./server/ -fuzz FuzzAuthEndpoint

# nginx -- needs linux/amd64; see the note below
cd <nginx> && git switch deckhouse-fuzzing
fuzz/build.sh libfuzzer && fuzz/run.sh -max_total_time=1800
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

# End-to-end tests of the user-authn module

The suite checks the `user-authn` module on a live cluster with [Chainsaw](https://kyverno.github.io/chainsaw/).
It signs local users in to Dex the way a browser does, calls kube-apiserver with the issued tokens
and checks what the module controller, webhooks and Dex do with users, groups and clients.

## Prerequisites

The suite runs as root on a master node with a cluster-admin kubeconfig.
It needs `chainsaw`, `task`, `kubectl`, `jq`, `curl`, `openssl`, `base32` and `perl`.
Password hashes are computed by `d8 iam user create --dry-run`,
or by glibc `crypt()` through `perl` when `d8` is absent.

The cluster must meet the following conditions:

- `ModuleConfig/global` sets `modules.publicDomainTemplate` and an HTTPS mode other than `Disabled`.
  Without them the module cannot be enabled, and the prerequisite step fails with this reason.
- The `dex-authenticator-app` case needs the `ingress-nginx` module with at least one `IngressNginxController`.
  Without a controller the case fails with an explicit message.
- The workload of the `dex-authenticator-app` case uses the image `registry.k8s.io/e2e-test-images/agnhost:2.53`,
  so the cluster must be able to pull from `registry.k8s.io`.
- The HA variant of the destructive suite needs two nodes on which the module may place its pods
  (nodes with the `system` role or, when there are none, any nodes).
  With one such node the case checks a single replica instead and says so in the log.

The master reaches Dex, `user-api` and the authenticator through the ClusterIP of their Services
and an ingress-nginx controller through the IP address of its pod.
`curl --resolve` keeps the public host names, so neither DNS records nor certificates are required.

## How the suite treats the cluster

Each test enables the module if needed with `testing/e2e/lib/module-enabled.sh ensure`.
Then it waits until the Module is `Ready`, the Deckhouse queue is empty and Dex answers.
`restore-modules.sh` puts back `spec.enabled` of `ModuleConfig/user-authn` after the suite with `module-enabled.sh restore`.

Tests that change module settings save the whole original `spec` of `ModuleConfig/user-authn`,
merge only their keys and replace the `spec` with the saved one in `cleanup`, even when the test fails.
After every change they wait until Dex runs with the new configuration.

The suite changes one ModuleConfig of another module.
The `dexclient-kubernetes-audience` case sets `apiserver.publishAPI.ingress.enabled` to `false`
in `ModuleConfig/control-plane-manager` when API publication through Ingress is enabled there.
The case restores the saved `spec` in `cleanup`, and `restore-modules.sh` restores it after a killed run.
While the case runs, the Kubernetes API is not published through Ingress.

All tests share the `e2e-authn` namespace.
The Taskfile creates it before Chainsaw and deletes it afterwards with `testing/e2e/lib/delete-namespace.sh`,
which deletes the Project of the same name when `multitenancy-manager` adopted the namespace.
Chainsaw ephemeral namespaces are not used, because `multitenancy-manager` refuses their deletion.
The namespace, ModuleConfig and Deckhouse queue helpers are shared by the module suites and live in `testing/e2e/lib`.
The Taskfile finds them through `E2E_LIB_DIR`: the CI bundle sets it, in a checkout it defaults to that directory.
Cluster-scoped objects are named with the `e2e-authn-` prefix.
`restore-modules.sh` runs before and after each suite.
It restores settings a killed run left changed, starts `user-authn-controller` again if the run left it scaled to zero,
and deletes leftover Users, Groups, UserOperations,
DexProviders, DexProviderChecks and ClusterRoleBindings with this prefix,
as well as the UserOperations that `user-api` created for users of the suite.
Before Chainsaw starts, the Taskfile also deletes and recreates the `e2e-authn` namespace.

Passwords are generated at runtime and kept in `/tmp/e2e-authn-state`, readable by root only.
Cookies, tokens and temporary files also stay there, and the directory is removed after the suite.
When a restore fails, only the records of the directory are kept for a repeated run, and the secrets are removed.
The scripts pass secrets to `curl` through files and never print passwords, hashes or tokens.

## Functional cases

The `tests` directory contains the following cases:

| Case | What it asserts |
| --- | --- |
| `static-user-login` | A User in a Group signs in with the password form and gets an ID token with its email, the group and the `kubernetes` audience. `kubectl auth whoami` with the token shows the email and the group. Without a binding listing pods is forbidden, with a ClusterRoleBinding to the group it is allowed. A wrong password gets HTTP 401. User status, the Dex Password and the UserAccount carry the group. |
| `user-lockout` | With `passwordPolicy.lockout.maxAttempts: 3` the third wrong password locks the account (HTTP 429 "Account temporarily locked"), and the right password is refused too. User `status.lock` reports `PasswordPolicyLockout`, the UserAccount is locked. `Unlock` lifts the lock; `Lock` sets `LockedByAdministrator`. |
| `useroperation-reset-password` | `ResetPassword` succeeds, wipes the hash from its spec, deletes the refresh tokens of the user and sets `requireResetHashOnNextSuccLogin`. The old password and refresh token stop working; the new password leads to the forced change page; the changed password signs in. A hash that is not bcrypt and an unknown user end in `Failed` with the reason. |
| `user-expire` | `spec.ttl: 2m` sets `status.expireAt` about two minutes after creation. Then the User is deleted and removed from the Group, together with its Password and UserAccount. A User without `ttl` stays. |
| `webhooks-validation` | The API server refuses with the exact messages: duplicate email (also in another case), uppercase or `system:` email, deprecated `spec.groups`, a change of the password hash, a duplicate Group name, a `system:` Group name, membership cycles, a zero lock duration, `ResetPassword` for an external target, a DexAuthenticator with an IP domain, without a publishing method, with a dot in the name or with a domain another DexAuthenticator serves. |
| `dex-authenticator-app` | An agnhost application behind ingress-nginx and a DexAuthenticator: an anonymous request is redirected to sign in; after the password form the session cookie gives HTTP 200 and the application receives `X-Auth-Request-Email`. |
| `dex-client-flow` | A DexClient gets the Secret with the labels and annotations from `secretMetadata` and both OAuth2Client objects. The ID token is issued for the client. A client with `allowedGroups` refuses a user outside them (HTTP 401 "Access denied"); a wrong client secret gets `invalid_client`. |
| `dexclient-kubernetes-access` | A DexClient with the annotation `dexclient.deckhouse.io/allow-access-to-kubernetes: "true"` becomes a trusted peer of the `kubernetes` client, and kube-apiserver accepts its token. An annotated DexAuthenticator also makes the module render the `kubernetes` client. |
| `dexclient-kubernetes-audience` | The annotation on a DexClient alone, without a DexAuthenticator, `publishAPI` or `kubeconfigGenerator`, makes the module render the `kubernetes` client with the DexClient among its trusted peers, and kube-apiserver accepts the token of the DexClient. Before that the case switches off `apiserver.publishAPI.ingress` of `control-plane-manager` and `kubeconfigGenerator` for its duration. When an annotated DexAuthenticator or DexClient of another module keeps the client rendered, the run logs `NOT PROVEN` and checks the end-to-end path only; `template_tests/kubernetes_oauth2client_test.go` is the regression guard of the DexClient-only path. |
| `conversion` | DexAuthenticator objects stored through v1, v2alpha1 and v1alpha1 read correctly in the other versions; DexProvider `teamNameField` changes case between v1alpha1 and v1. |
| `user-api-password-reset` | `POST /api/v1/password/reset` of `user-api` refuses a request without a token (401) and a hash that is not bcrypt (400), accepts a valid one (202) and creates a `ResetPassword` UserOperation with `initiatorType: self`. After it Dex refuses the old password and redirects the new one to the forced password change page. |
| `two-factor-auth` | With `staticUsers2FA` the first sign-in shows the enrollment QR code, refuses a wrong TOTP code and accepts the right one, computed from the key Dex stores. The next sign-in asks for a code only. `Reset2FA` deletes the key, so the user enrolls again. |
| `dex-provider-check` | The controller deletes a check whose provider does not exist (a check is named after its provider); a check of a disabled provider fails at `providerEnabled` without probing it. |
| `nested-groups` | A member of a Group nested in another Group belongs to both in User status, the Dex Password and the groups claim; after the parent Group is deleted only the direct group remains. |
| `refresh-token` | Each refresh returns a new ID token; a refresh after the reuse interval of 2 minutes rotates the refresh token. An administrator `Lock` deletes the refresh tokens of the user, so refresh fails with HTTP 400 even after `Unlock`, while a new sign-in works. |
| `rate-limit` | With `rateLimit` (1 request per minute, burst 2) seven successful sign-ins in a row from one address pass without a retry, because successful responses do not spend the budget; repeated failed sign-ins from one address get HTTP 429 "Too many requests" with `Retry-After`. |

## Destructive cases

The `tests-destructive` directory contains the `rollout-restart-under-probe` case.
With two nodes for the module the case enables `highAvailability` and waits for two replicas
of every component.
With one such node two replicas cannot be placed, so the case keeps one replica,
logs this choice and checks the rolling update of a single replica (`maxSurge: 1`) instead.
In this mode the VerticalPodAutoscaler of a component may also evict its only pod to apply new resources,
which makes the component unavailable until the replacement is ready.
A failed request that follows an `EvictedByVPA` event of a pod of the same component within 60 seconds
is reported in the log but does not fail the case; every other failed request does.
A probe then requests the following endpoints every 200 ms through their Services:

- the Dex discovery document;
- the authenticator `/ping`;
- the `user-api` `/healthz`.

Meanwhile Dex, a DexAuthenticator, `user-api` and `user-authn-controller` are restarted one by one.
Every request must get HTTP 200.
A UserOperation created while no controller replica runs must not be processed
and must complete once the controller is started again.
The original settings are restored afterwards.

## How to run

Run the suites from this directory:

```shell
task run
task run:destructive
task lint
```

The reports are written to `E2E_REPORTS_DIR` or, when it is not set, to `./reports`.
`run` takes about 25 minutes: three cases change the module settings, and each change restarts Dex twice.

## Cases that are not automated

The following planned cases are not automated:

- SAML, Kerberos, LDAP and Atlassian Crowd providers need an external identity provider,
  which the e2e cluster does not have.
- A custom logo is a cosmetic option of the login page with nothing measurable beyond a rendered file.
- A DexProviderCheck against an enabled OIDC provider that points at Dex itself is not run.
  An enabled provider enters the Dex configuration,
  and Dex does not start when an OIDC connector cannot discover its issuer at startup.
  The public Dex host is not guaranteed to resolve inside the cluster, so the case could stop Dex for the whole cluster.
- Publishing the API with a kubeconfig through Ingress is configured by the `control-plane-manager` module,
  not by `user-authn`.
  The suite does not check the published API.
  It only switches the publication off for the duration of `dexclient-kubernetes-audience`,
  as described in [How the suite treats the cluster](#how-the-suite-treats-the-cluster).

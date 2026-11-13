# Changelog v1.77.8

## Features


 - **[cloud-provider-yandex]** Add ru-central1-f zone support via explicit zone selection with a four-zone limit.
    Existing clusters are unaffected; the default zone set (ru-central1-a, ru-central1-b, ru-central1-d, ru-central1-e) is statically pinned. To use ru-central1-f, list it in `zones` instead of one of the default zones; the dropped zone's subnet is replaced by a new subnet in ru-central1-f and the nodes in it are recreated.
 - **[cloud-provider-yandex]** Add ru-central1-f zone support via explicit zone selection with a four-zone limit.

## Fixes


 - **[admission-policy-engine]** Pods with a Localhost AppArmor profile set in `securityContext` are no longer denied by the Baseline policy.
 - **[common]** Updated `golang.org/x/net`, `golang.org/x/text`, `google.golang.org/grpc` and `github.com/klauspost/compress` in the Kubernetes 1.31 CSI sidecars of CSE to fix CVEs.
 - **[control-plane-manager]** Fixed ControlPlaneOperation failures on clusters whose root CAs have a non-default subject.
 - **[dhctl]** The bootstrap terminal no longer shows glued lines, FAILED for bashible step attempts that recovered, or "Unhandled Error" from client-go.
 - **[registrypackages]** Fix AppArmor denying signals between processes of one container on Ubuntu kernels 7.0.0-41 and later.
    containerd restarts on all nodes. On nodes where the old AppArmor profile is already loaded, the fix applies after a node reboot.
 - **[user-authn]** Dex answers the refresh of a locked user, of a user deleted from the local store or LDAP, and of a claimed or expired refresh token with `invalid_grant` (HTTP 400), so that applications and a Dex that signs in through this one send the user to sign in.
    Dex pods restart. A client of Dex that expects HTTP 429 on the refresh of a locked user, HTTP 500 for a deleted one, or `invalid_request` for a claimed or expired refresh token gets HTTP 400 `invalid_grant`. The password and token exchange grants and the sign-in page answer as before.

## Chore


 - **[candi]** base images updated to v3.1.27

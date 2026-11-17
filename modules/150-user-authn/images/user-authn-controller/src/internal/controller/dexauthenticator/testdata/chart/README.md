# The objects the module chart rendered for DexAuthenticator objects

`authenticators.yaml` holds DexAuthenticator objects of the stored version, `v1`. Every other file is what the
templates `templates/dex-authenticator/{oauth2client,service,ingress,httproute}.yaml` of the module chart rendered
for them under one set of module settings, named in `chart_test.go`. A kind whose output is the same under several
sets is kept in one file: the Service in `certmanager.yaml` only, the Dex client and the HTTPRoute of
`disabled-old-annotations` in `certmanager.yaml` too. The test compares the objects the controller
makes for the same DexAuthenticator objects and settings with this output, so that the names, labels, annotations
and content of the objects stay what applications and modules already rely on.

The output was produced from the tree at commit `bb62926ae5`:

1. The objects were written in `v2alpha1` and converted to `v1` with `webhooks/conversion/dex_authenticator_conversion.py`.
2. The hook `hooks/get_dex_authenticator_crds.go` computed the values `userAuthn.internal.dexAuthenticatorCRDs`
   and `userAuthn.internal.dexAuthenticatorNames` from the `v2alpha1` objects.
3. The template tests library rendered the chart with these values.

The client secrets the hook generated are replaced with test values; the test does not compare them.

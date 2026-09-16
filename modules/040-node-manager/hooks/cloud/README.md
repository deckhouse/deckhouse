# cloud

Hooks of the cloud providers and CAPS (Cluster API provider for static nodes) teams. Their subject is a provider or a controller those teams own. node-manager only hosts the files.

- `generate_caps_webhook_certs` issues the self-signed TLS for the CAPS controller webhook and renews it before expiry.
- `sshcredentials_crd_cabundle_injection` writes the conversion webhook settings and the CA into the `SSHCredentials` CRD, so its two API versions convert through the CAPS webhook. The CRD manifest itself has no conversion section.
- `check_unmet_conditions` copies the unmet conditions reported by the cloud provider into the release requirements store, so a Deckhouse upgrade waits until the cloud side is ready.
- `yc_delete_preemptible_instances` deletes Yandex Cloud preemptible machines older than 24 hours at a moment of our choosing, before the cloud reclaims them at an inconvenient one.

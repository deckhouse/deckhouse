# caps

Hooks of CAPS, the Cluster API provider for static nodes, owned by the cloud providers team. node-manager only hosts the files.

- `generate_caps_webhook_certs` issues the self-signed TLS for the CAPS controller webhook and renews it before expiry.
- `sshcredentials_crd_cabundle_injection` writes the conversion webhook settings and the CA into the `SSHCredentials` CRD, so its two API versions convert through the CAPS webhook. The CRD manifest itself has no conversion section.

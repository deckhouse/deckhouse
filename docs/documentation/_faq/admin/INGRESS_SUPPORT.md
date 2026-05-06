---
title: Is the ingress-nginx module supported? Will it affect passing a PCI DSS audit?
subsystems:
  - network
lang: en
---

The ingress-nginx module remains supported by Deckhouse Kubernetes Platform for the entire platform support lifecycle and does not depend on the upstream project's status. The Deckhouse team tracks vulnerabilities in the controller and related components (NGINX, Lua modules, base images), delivers fixes in DKP releases, and provides a migration path toward Gateway API.

Flant is the vendor responsible for support. Together with ongoing module maintenance and vulnerability management, this allows passing a PCI DSS audit and meets expectations for vendor accountability for a platform component.

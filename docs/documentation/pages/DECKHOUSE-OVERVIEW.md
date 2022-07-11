---
title: Overview
permalink: en/deckhouse-overview.html
---

Welcome to the home page of the Deckhouse Kubernetes cluster management platform documentation! We recommend starting with the [Getting started]({% if site.mode == 'local' %}{{ site.urls[page.lang] }}{% endif %}/gs/) guide for those who haven't yet tried the platform. It provides step-by-step instructions for deploying the platform to any infrastructure.

Here are some tips on how to find what you need:
<ul>
<li>Note that the documentation is version-specific and may differ from version to version. You can select the Deckhouse version in the drop-down list at the top right of the page.</li>
<li>If you know what you're looking for exactly, use the search box at the top of the page.</li>
<li>Check <a href="revision-comparison.html">this list</a> if you are looking for information on a particular module.</li>
<li>The menu on the left is for searching by scope. Try to guess which section might contain what you are looking for.
  {% offtopic title="If in doubt, here is a brief description of the sections..." %}
  <div markdown="1">
  - Deckhouse — global settings and general information about the platform.
  - Kubernetes cluster — all things related to control-plane, integration with cloud providers, node management, network management, etc.
  - Accessing cluster — tools for accessing ([openvpn](modules/500-openvpn/)) and managing the cluster ([dashboard](modules/500-dashboard/)).
  - Network Load Balancing — [Nginx Ingress](modules/402-ingress-nginx/) and [Istio]({% if site.mode == 'local' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/documentation/v1/{% endif %}modules/110-istio/) features.
  - Monitoring — [Prometheus/Grafana](modules/300-prometheus/), [custom monitoring capabilities](modules/340-monitoring-custom/), and [logs collecting](modules/460-log-shipper/).
  - Autoscaling & Managing resources — all things related to Pod management and scaling.
  - Security — [authentication](modules/150-user-authn/), [authorization](modules/140-user-authz/) and [certificate management](modules/101-cert-manager/).
  - Storage — [integration with Ceph](modules/031-ceph-csi/), working [with a local storage](modules/031-local-path-provisioner/) on nodes, organizing [Linstor based storage](modules/041-linstor/).
  - Little things — [time synchronization](modules/470-chrony/), automatic [copying of secrets](modules/600-secret-copier/) by namespaces, and other amenities.
  - Bare Metal Support — modules for comfortable work with a cluster on bare metal servers.
  </div>
  {% endofftopic %}
</li>
</ul>
Can't find what you were looking for? Don't give up. Visit our [Telegram channel]({{ site.social_links[page.lang]['telegram'] }}) for help! There you will definitely find an answer to your problem.

Users of the Enterprise Edition of the platform can [email us](mailto:support@deckhouse.io) — we'll be sure to help.

Want to make Deckhouse better? Create an [issue](https://github.com/deckhouse/deckhouse/issues/), [discuss](https://github.com/deckhouse/deckhouse/discussions) your idea with us, or even [suggest a solution](https://github.com/deckhouse/deckhouse/blob/main/CONTRIBUTING.md)!

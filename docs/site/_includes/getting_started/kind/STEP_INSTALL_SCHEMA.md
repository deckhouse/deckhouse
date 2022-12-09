[kind](https://kind.sigs.k8s.io/) is a tool for running local Kubernetes clusters using container “nodes” and  was primarily designed for testing Kubernetes itself, but may be used for local development or CI.

Installing Deckhouse on kind, will allow you to get a Kubernetes cluster with Deckhouse installed in less than 10 minutes. It will allow you to get acquainted with Deckhouse main features quickly.

Please note that some features, such as [node management](/documentation/v1/modules/040-node-manager/) and [control plane management](/documentation/v1/modules/040-control-plane-manager/) will not work.

This guide covers installing Deckhouse in a **minimal** configuration, with Grafana based [monitoring](/documentation/v1/modules/300-prometheus/) enabled. To simplify, the [nip.io](https://nip.io ) service is used for working with DNS.

## Installation process

To install, you will need a personal computer that meets the following requirements:
- Operating system: macOS, Windows or Linux.
- At least 4GB of RAM.
- Installed container runtime (docker, containerd) and docker client.
- HTTPS access to the `registry.deckhouse.io` container image registry.

A Kubernetes cluster will be deployed on this computer, and Deckhouse will be installed into a cluster. 

You may choose the following variants of installation:
<ul>
<li>Go through the steps of the guide by yourself.</li>
<li>Use the <a href="https://github.com/deckhouse/deckhouse/blob/main/tools/kind-d8.sh">installation script</a> for Debian like Linux distributions or macOS:
  <ul>
  <li>Run the following command for installing Deckhouse Community Edition:<br/>
{% snippetcut selector="kind-install" %}
```shell
bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/kind-d8.sh)"
```
{% endsnippetcut %}
  </li>
  <li>Or run the following command for installing Deckhouse Enterprise Edition by providing a license key:<br/>
{% snippetcut selector="kind-install" %}
```shell
bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/kind-d8.sh)" -- --key <LICENSE_KEY>
```
{% endsnippetcut %}
  </li>
  <li>Go to the <a href="step5.html">final step</a> of the guide.</li>
  </ul>
</li>
</ul>

After installation is complete, you will be able to enable all the modules of interest on your own. Please, refer to the [documentation](/documentation/v1/) to learn more or reach out to the Deckhouse [community](/community/about.html).

[kind](https://kind.sigs.k8s.io/) is a tool for running local Kubernetes clusters using container “nodes” and  was primarily designed for testing Kubernetes itself, but may be used for local development or CI.

Installing Deckhouse on kind, will allow you to get a Kubernetes cluster with Deckhouse installed in less than 10 minutes. It will allow you to get acquainted with Deckhouse main features quickly.

Please note that some features, such as [node management](/{{ page.lang }}/documentation/v1/modules/040-node-manager/) and [control plane management](/{{ page.lang }}/documentation/v1/modules/040-control-plane-manager/) will not work.

This guide covers installing Deckhouse in a **minimal** configuration, with Grafana based [monitoring](/{{ page.lang }}/documentation/v1/modules/300-prometheus//) enabled. To simplify, the [nip.io](https://nip.io ) service is used for working with DNS.

After completing all the steps in this guide, you will be able to enable all the modules of interest on your own. Please, refer to the [documentation](/{{ page.lang }}/documentation/v1/) to learn more or reach out to the Deckhouse [community](/en/community/about.html).

## Installation process

You will need a personal computer of sufficient power with macOS, Windows or Linux and with Internet access. A Kubernetes cluster will be deployed on this computer, and Deckhouse will be installed into a cluster.

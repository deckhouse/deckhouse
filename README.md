<p align="center">
  <img src="https://raw.githubusercontent.com/deckhouse/deckhouse/main/docs/site/images/d8-small-logo.png"/>
</p>

<p align="center">
  <a href="https://t.me/deckhouse"><img src="https://img.shields.io/badge/telegram-chat-179cde.svg?logo=telegram" alt="Telegram chat"></a>
  <a href="https://twitter.com/deckhouseio"><img src="https://img.shields.io/twitter/follow/deckhouseio?label=%40deckhouseio&style=flat-square" alt="Twitter"></a>
  <a href="https://github.com/deckhouse/deckhouse/discussions"><img src="https://img.shields.io/github/discussions/deckhouse/deckhouse" alt="GH Discussions"/></a>
  <a href="CODE_OF_CONDUCT.md"><img src="https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg" alt="Contributor Covenant"></a>
  <a href="https://flow.deckhouse.io"><img src="https://img.shields.io/badge/releases-flow.deckhouse.io-blueviolet" alt="Releases"></a>
</p>


 Main features

<img align="right" width="200" height="270" src="docs/site/images/cncf-certified-kubernetes.png">

- NoOps: system software on the nodes, Kubernetes core software, Kubernetes platform components are automatically managed.
- SLA by design: availability can be guaranteed even without direct access to your infrastructure.
- Completely identical and infrastructure-agnostic clusters. Deploy on a public cloud of your choice (AWS, GCP, Microsoft Azure, OVH Cloud), self-hosted cloud solutions (OpenStack and vSphere), and even bare-metal servers.
- 100 % vanilla Kubernetes based on an upstream version of Kubernetes.
- Easy to start: you need a couple of CLI commands and 8 minutes to get production-ready Kubernetes.
- A fully-featured platform. Many features *(check the diagram below)* — carefully configured & integrated — are available right out of the box.

_Deckhouse Platform [has passed](https://landscape.cncf.io/card-mode?category=certified-kubernetes-distribution,certified-kubernetes-hosted,certified-kubernetes-installer&grouping=category&selected=flant-deckhouse) the CNCF Certified Kubernetes Conformance Program certification for Kubernetes 1.23—1.27._

A brief overview of essential Deckhouse Platform features, from infrastructure level to the platform:

<img src="https://raw.githubusercontent.com/deckhouse/deckhouse/main/docs/site/images/diagrams/structure.svg">

## CE vs. EE

While Deckhouse Platform CE is available free as an Open Source, EE is a commercial version of the platform that can be purchased with a paid subscription. EE's source is also open, but it's neither Open Source nor free to use.

EE brings many additional features that extend the basic functionality provided in CE. They include OpenStack & vSphere integration, Istio service mesh, multitenancy, enterprise-level security, BGP support, instant autoscaling, local DNS caching, and selectable timeframe for the platform's upgrades.

Deckhouse Platform CE is freely available for everyone. Deckhouse Platform EE can be accessed via 30-days tokens issued via [Deckhouse website](https://deckhouse.io/).

# Architecture

Deckhouse Platform follows the upstream version of Kubernetes, using that as a basis to build all of its features and configurations on. The added functionality is implemented via two building blocks:

- [shell-operator](https://github.com/flant/shell-operator) — to create Kubernetes operators *(please check the [KubeCon NA 2020 talk](https://www.youtube.com/watch?v=we0s4ETUBLc) for details)*;
- [addon-operator](https://github.com/flant/addon-operator) — to pack these operators into modules and manage them.

# Trying Deckhouse

Please, refer to the project's [Getting started](https://deckhouse.io/gs/) to begin your journey with Deckhouse Platform. Choose the cloud provider or bare-metal option for your infrastructure and follow the relevant step-by-step instructions to deploy your first Deckhouse Kubernetes cluster.

If anything works in an unexpected manner or you have any questions, feel free to contact us via GitHub Issues / Discussions or reach a wider [community of Deckhouse users](#online-community) in Telegram and other resources.

# Online community

In addition to common GitHub features, here are some other online resources related to Deckhouse:

* [Twitter](https://twitter.com/deckhouseio) to stay informed about everything happening around Deckhouse;
* [Telegram chat](https://t.me/deckhouse) to discuss (there's a dedicated [Telegram chat in Russian](https://t.me/deckhouse_ru) as well);
* [Deckhouse blog](https://blog.deckhouse.io/) to read the latest articles about Deckhouse.
* Check our [work board](https://github.com/orgs/deckhouse/projects/2) and [roadmap](https://github.com/orgs/deckhouse/projects/6) for more insights.

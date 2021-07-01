[Deckhouse Platform](https://deckhouse.io/) is an operator which creates homogeneous Kubernetes clusters anywhere and fully manages them. It supplies all necessary addons to provide observability, security, and service mesh. It comes in Enterprise Edition (EE) and Community Edition (CE).

# Main features
- NoOps: system software on the nodes, Kubernetes core software, Kubernetes platform components are automatically managed.
- SLA by design: availability guarantees even without direct access to your infrastructure.
- Fully identical and infrastructure-agnostic clusters. Deploy on a public cloud of your choice (AWS, GCP, Microsoft Azure, OVH Cloud), self-hosted cloud solutions (OpenStack and vSphere), and even bare-metal servers.
- 100 % vanilla Kubernetes based upstream version of Kubernetes.
- Easy to start: you need a couple of CLI commands and 8 minutes to get production-ready Kubernetes.
- Feature-complete platform. Many features — carefully configured & integrated — are available right out of the box.

## CE vs. EE
While Deckhouse Platform CE is available free as an Open Source, EE is a commercial version of the platform that can be purchased with a paid subscription. EE's source is also open, but it's neither Open Source nor free to use.

EE brings many additional features that extend the basic functionality provided in CE. They include OpenStack & vSphere integration, Istio service mesh, multitenancy, enterprise-level security, BGP support, instant autoscaling, local DNS caching, and selectable timeframe for the platform's upgrades.

# Architecture
Deckhouse Platform follows the upstream version of Kubernetes, building all its features and configurations on top of it. The added functionality is implemented via two building blocks:

- [shell-operator](https://github.com/flant/shell-operator) — to create Kubernetes operators *(please check the [KubeCon NA 2020 talk](https://www.youtube.com/watch?v=we0s4ETUBLc) for details)*;
- [addon-operator](https://github.com/flant/addon-operator) — to pack these operators into modules and manage them.

# Current status

Deckhouse Platform has a vast history of being used internally in Flant and is ready for production. Its beta testing started in May'21 when the first public demo tokens were issued. By the end of Jun'21, it was tested by 300+ engineers, and its source code went public via GitHub.

Deckhouse Platform CE is now freely available for everyone. Deckhouse Platform EE can be accessed via 30-days tokens issued via [Deckhouse website](https://deckhouse.io/). Please, follow our [getting started](https://deckhouse.io/en/getting_started.html) instructions to begin your journey with Deckhouse Platform.

# Online community
In addition to common GitHub features, here are some other online resources related to Deckhouse:

* [Twitter](https://twitter.com/deckhouseio) to stay informed about everything happening around Deckhouse;
* [Telegram chat](https://t.me/deckhouse) to discuss (there's a dedicated [Telegram chat in Russian](https://t.me/deckhouse_ru) as well);
* Flant's [tech blog](https://blog.flant.com/tag/deckhouse/) to read posts related to Deckhouse.

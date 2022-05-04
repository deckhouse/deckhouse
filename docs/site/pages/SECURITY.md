---
title: Deckhouse Security
description: Deckhouse Security
permalink: en/security.html
layout: default
toc: false
anchors_disabled: true
---

{::options parse_block_html="false" /}

<section class="intro">
  <div class="intro__content container">
    <h1 class="intro__title text_lead text_alt">
      Deckhouse Security
    </h1>
    <div class="intro__row">
      <div>
        <p class="text text_big">
          We use proven Open Source tools and best DevSecOps practices to increase the security of the cluster and the applications deployed in it. The platform implements advanced authentication and authorization mechanisms, secure interaction of components, encryption, auditing and other crucial functionality.
        </p>
      </div>
    </div>
  </div>
  <div class="block__content block__columns block__columns_top container">
    <div>
      <h2 class="text text_h2">
        CIS Benchmarks
      </h2>
      <p class="text text_big">
        Deckhouse meets
        <a href="https://www.cisecurity.org/benchmark/kubernetes" target="_blank">CIS Kubernetes Benchmark recommendations</a>*.
        Security measures are implemented both at the component level and at the platform level.
        For example, you can restrict network access to the necessary interfaces only, prohibit anonymous access,
        use certificates, set permissions for files and directories.
      </p>
      <p class="text text_small">
        * CIS Kubernetes Benchmark is a set of guidelines for creating a reliable security environment for Kubernetes-based software.
      </p>
    </div>
    <div>
      <h2 class="text text_h2">
        SELinux
      </h2>
      <p class="text text_big">
        <a href="https://github.com/SELinuxProject" target="_blank">Security-Enhanced Linux (SELinux)</a>*
        is a standard for securing Linux distributions. 
        You can forcefully activate the SELinux mode in distributions that are used with Deckhouse.
      </p>
      <p class="text text_small">
        * SELinux defines access policies for applications, processes and files.
      </p>
    </div>
  </div>
</section>

<section class="features">
  <div class="container">
    <h2 class="features__title text_lead text_alt">
      Tools
    </h2>
    <p class="text text_big">
      Deckhouse provides a set of solutions for secure authentication, authorization, network policy management, ordering TLS certificates, and more.
    </p>
  </div>

  <div class="features__item features__item_even">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Federated Authentication Provider
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Deckhouse provides a pre-installed federated authentication provider based on Dex (Identity Provider, IdP).
        </li>
        <li>
          It is integrated with Kubernetes and all service components.
        </li>
        <li>
          It can be integrated with the application if it supports OIDC.
        </li>
        <li>
          The oauth2-proxy operator facilitates convenient interaction with the Ingress controller.
        </li>
        <li>
          You can create users directly in the cluster or connect users of external authentication systems: GitHub, GitLab, OIDC, LDAP.
        </li>
      </ul>
    </div>
  </div>

  <div class="features__item features__item_odd">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Authorization<br>
          <small>simplified RBAC</small>
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Deckhouse provides a simplified and more user-friendly version of RBAC in Kubernetes. It has seven ready-made roles that are suitable for any use-cases. This reduces the chance of error and makes it easier to configure authorization policies.
        </li>
        <li>
          If necessary, you can increase the number of roles using the standard Kubernetes RBAC tools.
        </li>
      </ul>
    </div>
  </div>

  <div class="features__item features__item_even">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          And more
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          The network policy management module. A simple and reliable system with rules that are independent of the type of installation and the CNI used.
        </li>
        <li>
          Kubernetes event auditing to account for cluster operations and error analysis.
        </li>
        <li>
          The cert-manager module. This module can order third-party TLS certificates and issue self-signed ones. It keeps certificates up to date and automatically reissues them.
        </li>
      </ul>
    </div>
  </div>

  <div class="features__item features__item_odd">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Coming soon
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Multitenancy
        </li>
        <li>
          Integration with HashiCorp Vault
        </li>
        <li>
          Integration with OpenPolicyAgent
        </li>
      </ul>
    </div>
  </div>

</section>

<section class="features">
  <div class="container">
    <h2 class="features__title text_lead text_alt">
      Building components
    </h2>
  </div>

  <div class="features__item features__item_even">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Guidelines
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Docker images for all platform components can only be downloaded from the Deckhouse repository.
        </li>
        <li>
          Only the necessary binaries from the original image files created by the software authors are used.
        </li>
        <li>
          All dependencies related to the original images as well as the digest of the image are strictly pre-defined. The resulting image is built based on our base image.
        </li>
        <li>
          In the vast majority of cases, we use the Alpine (the most secure Linux distribution) to build the base image.
        </li>
        <li>
          Updates of the basic images are performed seamlessly. Kubernetes gets updated automatically according to the regulations.
        </li>
      </ul>
    </div>
  </div>

  <div class="features__item features__item_odd">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          How it works
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          We carefully choose the software. We use only the solutions that have proven to be reliable based on our own and the Open Source community experience.
        </li>
        <li>
          Most checks are performed in an automated manner using linters. For example, they make sure that the Dockerfile configuration is correct and prohibit the use of third-party images.
        </li>
        <li>
          We constantly monitor new CVEs for all the software used. All the Sn level (and more severe) vulnerabilities are handled within three hours, while Sn-Sk level vulnerabilities are handled within 24 hours.
        </li>
      </ul>
    </div>
  </div>

</section>

<section class="block container">
  <div class="block__content">
    <h2 class="text text_h1">
      An example of Dockerfile for the kube-dns module*
    </h2>
<div markdown="1" class="docs">
```docker
# Based on https://github.com/coredns/coredns/blob/master/Dockerfile
ARG BASE_ALPINE
FROM coredns/coredns:1.6.9@sha256:40ee1b708e20e3a6b8e04ccd8b6b3dd8fd25343eab27c37154946f232649ae21 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /coredns /coredns
ENTRYPOINT [ "/coredns" ]
```
<p class="text">
  * The module installs CoreDNS components for managing DNS in the Kubernetes cluster.
</p>
</div>
  </div>
</section>

<section class="features">
  <div class="container">
    <h2 class="features__title text_h1">
      Configuring components and their interaction
    </h2>
  </div>

  <div class="features__item features__item_even">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Guidelines
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Each component runs with a minimal set of privileges in Kubernetes that is sufficient for its operation ("minimum RBAC").
        </li>
        <li>
          Components never have root privileges. Exceptions are explicitly defined in the permission list.
        </li>
        <li>
          The root file system is read-only (except for a small number of specific directories).
        </li>
        <li>
          Local ports of all Deckhouse components are secured with TLS encryption and authentication.
        </li>
        <li>
          Additional authentication and authorization requests to the Kubernetes API are cached and do not affect performance.
        </li>
      </ul>
    </div>
  </div>

  <div class="features__item features__item_odd">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Authorization<br>
          <small>simplified RBAC</small>
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Linters check that RBAC rights are defined in a specific file of each Deckhouse module explicitly and unambiguously. This provides a single point of control.
        </li>
        <li>
          Service Account, role, rolebinding, etc., names are strictly regulated, thus protecting against human error.
        </li>
        <li>
          Authentication between cluster components is carried out in two ways: via TLS or bearer tokens. Authorization is performed via Kubernetes mechanisms (SubjectAccessReview).
        </li>
      </ul>
    </div>
  </div>
</section>

<section class="block container">
  <div class="block__content">
    <p class="text text_big">
      <strong>Example:</strong> suppose you use Prometheus for monitoring the cluster. It collects data from all components. Each component has a port for connecting the monitoring service. Prometheus uses an individual SSL certificate to connect to this port.
    </p>
    <p class="text text_big">
      Upon receiving a request from Prometheus, the component authenticates it by checking if the Prometheus certificate is signed by the Kubernetes Certificate Authority. Then it performs authorization by requesting SubjectAccessReview. This mechanism ensures that only Prometheus can connect to the monitoring ports.
    </p>
  </div>
</section>

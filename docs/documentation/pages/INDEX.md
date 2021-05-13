---
title: "Home"
permalink: en/
toc: false
comparable: false
---


{::options parse_block_html="false" /}
<div class="docs-cards">

  <div class="docs-card">
    <h3 class="docs-card__title">
      <a href="features/core.html">
        The core subsystem
      </a>
    </h3>
    <p>The core of Deckhouse. Provides basic functionality and manages the update policy.</p>
    <p><a href="features/core-faq.html">FAQ</a></p>
    <!--
    <ul>
    <li>Как автоматически менять канал обновлений</li>
    <li>Как узнать параметры модулей в текущей версии кластера</li>
    </ul>
    -->
  </div>

  <div class="docs-card">
    <h3 class="docs-card__title">
      <a href="features/auth.html">
        The auth subsystem
      </a>
    </h3>
    <p>Secure cluster using. Integration with external directories. User management.</p>
    <!--
    <p><a href="features/auth-faq.html">FAQ</a></p>
    <ul>
    <li>Настройка аутентификации через мой GitLab/Ldap/BitBucket/ActiveDirectory/ другой провайдер</li>
    <li>Как завести пользователя через CRD.</li>
    <li>Как дать доступ к API-серверу публично, через VPN, конкретным сетям.</li>
    <li>Использование отдельного CA для работы control-plane.</li>
    <li>Ограничить права пользователям конкретными namespace</li>
    </ul>
    -->
  </div>

  <div class="docs-card">
    <h3 class="docs-card__title">
      <a href="features/candi.html">
        The CandI subsystem
      </a>
    </h3>
    <p>Manages Kubernetes control-plane and configures nodes. Provides a ready-to-use, up-to-date cluster on any infrastructure.</p>
    <!--
    <p><a href="features/candi-faq.html">FAQ</a></p>
    <ul>
    <li>Как управлять шедулингов ресурсов Deckhouse.</li>
    <li>Как из single-мастер кластера сделать multi-мастер.</li>
    <li>Как добавить секрет доступа к приватному Docker-registry.</li>
    <li>Как распространить секрет во все namespace кластера.</li>
    </ul>
    -->
  </div>

  <div class="docs-card">
    <h3 class="docs-card__title">
      <a href="features/marm.html">
        The marm subsystem
      </a>
    </h3>
    <p>Customizable monitoring based on Prometheus/Grafana with ready-made templates for popular applications. Scaling with monitoring in mind.</p>
    <!--
    <p><a href="features/marm-faq.html">FAQ</a></p>
    <ul>
    <li>Как кастомизировать Grafana и почему она stateless?</li>
    <li>Как замониторить свое приложение и собирать его метрики.</li>
    <li>Как добавить свои Dashboard</li>
    <li>Как мониторить доступность произвольных узлов.</li>
    <li>Как подключить свой alert-manager</li>
    <li>Как выключить longterm prometheus?</li>
    <li>Как настроить хранилище для Prometheus</li>
    <li>Как зашедулить что-то (Prometheus/Grafana, Dashboard и т.п.) на отдельный узел.</li>
    <li>Как добавить кастомный плагин в Grafana.</li>
    <li>Как настроить хранилище и параметры ротации данных Prometheus/Longterm Prometheus.</li>
    <li>Как настроить выделенную ноду для работы мониторинга.</li>
    <li>Как отключить Longterm Prometheus.</li>
    </ul>
    -->
  </div>

  <div class="docs-card">
    <h3 class="docs-card__title">
      <a href="modules/101-cert-manager/">
        Must Have module collection
      </a>
    </h3>
    <p>Installs Dashboard and Nginx-based Ingress. Manages SSL certificates.</p>
    <!--
    <p><a href="./">FAQ</a></p>
    <ul>
      <li>Как выдать выдать админские права в Dashboard.</li>
      <li>Как Ограничить доступ к web-ресурсам по IP allowlist’у</li>
      <li>Как Использовать свой Wildcard-сертификат для работы web-интерфейса модулей</li>
      <li>Как настроить автоматическую работу с сертификатами LetsEncrypt/CloudFlare/Route53/Google</li>
    </ul>
    -->
  </div>

  <div class="docs-card">
    <h3 class="docs-card__title">
      <a href="modules/050-network-policy-engine/">
        Extended Networking module collection
      </a>
    </h3>
    <p>Access to a cluster via VPN, network access policies, DNS booster, and Istio.</p>
    <!--
    <p><a href="./">FAQ</a></p>
    <ul>
    <li>Настройка доступа в кластер по VPN.</li>
    <li>Настройка доступа в кластер по VPN через.</li>
    <li>Как дать доступ к ресурсу внутри кластера через VPN.</li>
    <li>Как разрешить подам только доступ к внешним ресурсам и внутри своего namespace, но запретить остальное.</li>
    </ul>
    -->
  </div>

  <div class="docs-card">
    <h3 class="docs-card__title">
      <a href="modules/380-metallb/">
        Bare Metal Compatibility module collection
      </a>
    </h3>
    <p>Balancing in bare-metal clusters based on metallb and keepalived. Cluster network gateway.</p>
    <!--
    <p><a href="./">FAQ</a></p>
    <ul>
      <li>Как настроить плавающий IP.</li>
    </ul>
    -->
  </div>

</div>
{::options parse_block_html="true" /}

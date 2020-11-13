---
title: Deckhouse
permalink: /
layout: default
---

{::options parse_block_html="false" /}
<div class="page__container">
    <div class="welcome__content">
<!--        <h1 class="welcome__title">-->
<!--            Deckhouse дает надежный фундамент для ваших приложений — актуальный, <span>одинаково</span> работающий <span>на любой инфраструктуре</span> «ванильный» Kubernetes.-->
<!--        </h1>-->
<!--<p>Гибкое решение, состоящее из нескольких связанных подсистем и модулей, настраиваемое в одном месте (идеально для GitOps).</p>-->
    </div>
<div class="main-page__features-container">
  <ul class="main-page__features-list">
    <li class="main-page__feature">
      <div class="card-benefits">
        <div class="card-benefits__inner">
          <div class="card-benefits__icon-container">
            <svg class="icon card-benefits__icon" width="68" height="68" aria-hidden="true">
            </svg>
          </div>
          <div class="card-benefits__header">
            <a href="/features/core.html"><h3 class="title card-benefits__title title--subtitle">Подсистема core</h3></a>
          <div class="text card-benefits__text">
            <p>Ядро Deckhouse. Обеспечивает базовый функционал и управляет политикой обновления.</p>
            <p class="card-benefits__faq"><a href="/features/core-faq.html">FAQ</a></p>
<!--            <ul class="main-page__usercases-list">-->
<!--            <li>Как автоматически менять канал обновлений</li>-->
<!--            <li>Как узнать параметры модулей в текущей версии кластера</li>-->
<!--            </ul>-->
          </div>
          </div>
        </div>
      </div>
    </li>
    <li class="main-page__feature">
      <div class="card-benefits">
        <div class="card-benefits__inner">
          <div class="card-benefits__icon-container">
            <svg class="icon card-benefits__icon" width="68" height="68" aria-hidden="true">
            </svg>
          </div>
          <div class="card-benefits__header">
            <a href="/features/auth.html"><h3 class="title card-benefits__title title--subtitle">Подсистема auth</h3></a>
          <div class="text card-benefits__text">
            <p>Безопасное совместное использование кластера. Интеграция с внешними каталогами. Управление пользователями.</p>
<!--            <p class="card-benefits__faq"><a href="/features/auth-faq.html">FAQ</a></p>-->
<!--            <ul class="main-page__usercases-list">-->
<!--            <li>Настройка аутентификации через мой GitLab/Ldap/BitBucket/ActiveDirectory/ другой провайдер</li>-->
<!--            <li>Как завести пользователя через CRD.</li>-->
<!--            <li>Как дать доступ к API-серверу публично, через VPN, конкретным сетям.</li>-->
<!--            <li>Использование отдельного CA для работы control-plane.</li>-->
<!--            <li>Ограничить права пользователям конкретными namespace</li>-->
<!--            </ul>-->
          </div>
          </div>
        </div>
      </div>
    </li>
    <li class="main-page__feature">
      <div class="card-benefits">
        <div class="card-benefits__inner">
          <div class="card-benefits__icon-container">
            <svg class="icon card-benefits__icon" width="59" height="59" aria-hidden="true">
            </svg>
          </div>
          <div class="card-benefits__header">
            <a href="/features/candi.html"><h3 class="title card-benefits__title title--subtitle">Подсистема CandI</h3></a>
          </div>
          <div class="text card-benefits__text">
            <p>Управляет control-plane Kubernetes, настраивает узлы. Дает готовый к работе, актуальный кластер на любой инфраструктуре.</p>
<!--            <p class="card-benefits__faq"><a href="/features/candi-faq.html">FAQ</a></p>-->
          </div>
<!--                       <ul class="main-page__usercases-list"> -->
<!--             <li>Как управлять шедулингов ресурсов Deckhouse.</li> -->
<!--             <li>Как из single-мастер кластера сделать multi-мастер.</li> -->
<!--             <li>Как добавить секрет доступа к приватному Docker-registry.</li> -->
<!--             <li>Как распространить секрет во все namespace кластера.</li> -->
<!--             </ul> -->
        </div>
      </div>
    </li>
    <li class="main-page__feature">
      <div class="card-benefits">
        <div class="card-benefits__inner">
          <div class="card-benefits__icon-container">
            <svg class="icon card-benefits__icon" width="59" height="59" aria-hidden="true">
            </svg>
          </div>
          <div class="card-benefits__header">
            <a href="/features/marm.html"><h3 class="title card-benefits__title title--subtitle">Подсистема marm</h3></a>
          <div class="text card-benefits__text">
            <p>Настраиваемый мониторинг на базе Prometheus/Grafana с готовыми шаблонами для популярных приложений. Масштабирование с учетом мониторинга.</p>
<!--            <p class="card-benefits__faq"><a href="/features/marm-faq.html">FAQ</a></p>-->
          </div>
<!--            <ul class="main-page__usercases-list">-->
<!--            <li>Как кастомизировать Grafana и почему она stateless?</li>-->
<!--            <li>Как замониторить свое приложение и собирать его метрики.</li>-->
<!--            <li>Как добавить свои Dashboard</li>-->
<!--            <li>Как мониторить доступность произвольных узлов.</li>-->
<!--            <li>Как подключить свой alert-manager</li>-->
<!--            <li>Как выключить longterm prometheus?</li>-->
<!--            <li>Как настроить хранилище для Prometheus</li>-->
<!--            <li>Как зашедулить что-то (Prometheus/Grafana, Dashboard и т.п.) на отдельный узел.</li>-->
<!--            <li>Как добавить кастомный плагин в Grafana.</li>-->
<!--            <li>Как настроить хранилище и параметры ротации данных Prometheus/Longterm Prometheus.</li>-->
<!--            <li>Как настроить выделенную ноду для работы мониторинга.</li>-->
<!--            <li>Как отключить Longterm Prometheus.</li>-->
<!--            </ul>-->
          </div>
        </div>
      </div>
    </li>
    <li class="main-page__feature">
      <div class="card-benefits">
        <div class="card-benefits__inner">
          <div class="card-benefits__icon-container">
            <svg class="icon card-benefits__icon" width="62" height="58" aria-hidden="true">
            </svg>
          </div>
          <div class="card-benefits__header">
            <a href="/modules/101-cert-manager"><h3 class="title card-benefits__title title--subtitle">Набор модулей — Must Have Collection</h3></a>
          <div class="text card-benefits__text">
            <p>Устанавливает Dashboard, Ingress на базе Nginx. Управляет SSL-сертификатами.</p>
<!--            <p class="card-benefits__faq"><a href="./">FAQ</a></p>-->
<!--            <ul class="main-page__usercases-list">-->
<!--            <li>Как выдать выдать админские права в Dashboard.</li>-->
<!--<li>Как Ограничить доступ к web-ресурсам по IP allowlist’у</li>-->
<!--<li>Как Использовать свой Wildcard-сертификат для работы web-интерфейса модулей</li>-->
<!--<li>Как настроить автоматическую работу с сертификатами LetsEncrypt/CloudFlare/Route53/Google</li>-->
<!--</ul>-->
          </div>
          </div>
        </div>
      </div>
    </li>
    <li class="main-page__feature">
      <div class="card-benefits">
        <div class="card-benefits__inner">
          <div class="card-benefits__icon-container">
            <svg class="icon card-benefits__icon" width="68" height="68" aria-hidden="true">
            </svg>
          </div>
          <div class="card-benefits__header">
              <a href="/modules/050-network-policy-engine"><h3 class="title card-benefits__title title--subtitle">Набор модулей — Extended Networking Collection</h3></a>
          <div class="text card-benefits__text">
            <p>Доступ в кластер через VPN, сетевые политики доступа, ускорение работы с DNS (скоро еще и Istio).</p>
<!--            <p class="card-benefits__faq"><a href="./">FAQ</a></p>-->
<!--            <ul class="main-page__usercases-list">-->
<!--            <li>Настройка доступа в кластер по VPN.</li>-->
<!--            <li>Настройка доступа в кластер по VPN через.</li>-->
<!--            <li>Как дать доступ к ресурсу внутри кластера через VPN.</li>-->
<!--            <li>Как разрешить подам только доступ к внешним ресурсам и внутри своего namespace, но запретить остальное.</li>-->
<!--            </ul>-->
          </div>
          </div>
        </div>
      </div>
    </li>
    <li class="main-page__feature">
      <div class="card-benefits">
        <div class="card-benefits__inner">
          <div class="card-benefits__icon-container">
            <svg class="icon card-benefits__icon" width="68" height="68" aria-hidden="true">
            </svg>
          </div>
          <div class="card-benefits__header">
            <a href="/modules/380-metallb"><h3 class="title card-benefits__title title--subtitle">Набор модулей — Bare Metal Compatibility Collection</h3></a>
          <div class="text card-benefits__text">
            <p>Балансировка в bare-metal кластерах на базе metallb и keepalived. Организация сетевого шлюза в кластере.</p>
<!--            <p class="card-benefits__faq"><a href="./">FAQ</a></p>-->
<!--            <ul class="main-page__usercases-list">-->
<!--            <li>Как настроить плавающий IP.</li>-->
<!--            </ul>-->
          </div>
          </div>
        </div>
      </div>
    </li>
  </ul>
</div>
</div>

{::options parse_block_html="true" /}

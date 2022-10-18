---
title: Безопасность Deckhouse
description: Безопасность Deckhouse
permalink: ru/security.html
layout: default
toc: false
lang: ru
anchors_disabled: true
---

{::options parse_block_html="false" /}

<section class="intro">
  <div class="intro__content container">
    <h1 class="intro__title text_lead text_alt">
      Безопасность Deckhouse
    </h1>
    <div class="intro__row">
      <div>
        <p class="text text_big">
          Чтобы повысить защищенность кластера и развернутых в нем приложений, мы используем проверенные Open
          Source-инструменты и лучшие практики DevSecOps. В платформе реализованы продвинутые механизмы аутентификации и
          авторизации, безопасное взаимодействие компонентов, шифрование, аудит и другие важные функции.
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
        Deckhouse соответствует
        <a href="https://www.cisecurity.org/benchmark/kubernetes" target="_blank">рекомендациям CIS Kubernetes Benchmark</a>*.
        Это реализовано на уровне компонентов и платформы в целом. Например, можно указывать сетевые привязки
        только к нужным интерфейсам, запретить анонимный доступ, использовать сертификаты, права на файлы и каталоги.
      </p>
      <p class="text text_small">
        * CIS Kubernetes Benchmark — набор рекомендаций по созданию надежной системы безопасности для ПО на базе Kubernetes.
      </p>
    </div>
    <div>
      <h2 class="text text_h2">
        SELinux
      </h2>
      <p class="text text_big">
        <a href="https://github.com/SELinuxProject" target="_blank">Security-Enhanced Linux (SELinux)</a>*
        — стандарт для защиты Linux-дистрибутивов. В дистрибутивах, которые используются в Deckhouse,
        можно активировать принудительное включение режима SELinux.
      </p>
      <p class="text text_small">
        * SELinux определяет политики доступа к приложениям, процессам и файлам.
      </p>
    </div>
  </div>
</section>

<section class="features">
  <div class="container">
    <h2 class="features__title text_lead text_alt">
      Инструменты
    </h2>
    <p class="text text_big">
      Deckhouse предоставляет набор решений для безопасной аутентификации, авторизации, управления сетевыми политиками,
      заказа TLS-сертификатов и не только.
    </p>
  </div>

  <div class="features__item features__item_even">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Федеративный провайдер аутентификации
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Предустановленный федеративный провайдер аутентификации на базе Dex (Identity Provider, IdP).
        </li>
        <li>
          Интегрирован с Kubernetes и всеми служебными компонентами.
        </li>
        <li>
          Возможна интеграция с приложением, если оно поддерживает OIDC.
        </li>
        <li>
          Оператор oauth2-proxy поддерживает удобное взаимодействие с Ingress-контроллером.
        </li>
        <li>
          Можно создавать пользователей прямо в кластере, а также подключать пользователей внешних систем
          аутентификации: GitHub, GitLab, OIDC, LDAP.
        </li>
      </ul>
    </div>
  </div>

  <div class="features__item features__item_odd">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Авторизация<br>
          <small>упрощенный RBAC</small>
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Deckhouse предлагает более простую и удобную версию RBAC Kubernetes — 7 готовых ролей, которые подходят для
          любых практических сценариев. Это снижает вероятность ошибки и облегчает настройку политик авторизации.
        </li>
        <li>
          Если необходимо, можно расширить количество ролей через обычные средства RBAC Kubernetes.
        </li>
      </ul>
    </div>
  </div>

  <div class="features__item features__item_even">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          А также
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Модуль управления сетевыми политиками. Простая и надежная система с правилами, которые не зависят от типа
          инсталляции и используемого CNI.
        </li>
        <li>
          Аудит событий Kubernetes для учета операций в кластере и анализа ошибок.
        </li>
        <li>
          Модуль cert-manager. Поддерживает заказ сторонних TLS-сертификатов и выпуск самоподписанных. Актуализирует и
          автоматически перевыпускает сертификаты.
        </li>
      </ul>
    </div>
  </div>

  <div class="features__item features__item_odd">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Скоро
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Multitenancy
        </li>
        <li>
          Интеграция с HashiCorp Vault
        </li>
        <li>
          Интеграция с OpenPolicyAgent
        </li>
      </ul>
    </div>
  </div>

</section>

<section class="features">
  <div class="container">
    <h2 class="features__title text_lead text_alt">
      Сборка компонентов
    </h2>
  </div>

  <div class="features__item features__item_even">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Правила
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Docker-образы для всех компонентов платформы можно скачивать только из репозитория Deckhouse.
        </li>
        <li>
          Из оригинальных образов от разработчиков ПО используются только нужные бинарные файлы.
        </li>
        <li>
          Все зависимости на оригинальные образы, а также digest образа строго прописаны. Результирующий образ
          собирается из нашего базового образа.
        </li>
        <li>
          Для сборки базового образа почти всегда используется Alpine — самый безопасный дистрибутив Linux.
        </li>
        <li>
          Базовые образы обновляются бесшовно. Kubernetes обновляется автоматически в соответствии с регламентом.
        </li>
      </ul>
    </div>
  </div>

  <div class="features__item features__item_odd">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Как это реализовано
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Тщательно выбираем софт. Используем только те решения, которые доказали свою надежность в наших проектах и в
          Open Source-сообществе.
        </li>
        <li>
          Большинство проверок автоматизированы, за это отвечают линтеры. Например, они отслеживают корректную
          конфигурацию Dockerfile’ов и запрещают использовать сторонние образы.
        </li>
        <li>
          Отслеживаем новые CVE по всему используемому ПО. Разбираем инциденты уровня Sn и выше в течение 3 часов,
          уровня Sn-Sk — в течение 24 часов.
        </li>
      </ul>
    </div>
  </div>

</section>

<section class="block container">
  <div class="block__content">
    <h2 class="text text_h1">
      Пример Dockerfile для модуля kube-dns*
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

</div>
<p class="text">
  * Модуль устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes.
</p>
  </div>
</section>

<section class="features">
  <div class="container">
    <h2 class="features__title text_h1">
      Настройка и взаимодействие компонентов
    </h2>
  </div>

  <div class="features__item features__item_even">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Правила
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Каждый компонент запускается с минимальными правами доступа в Kubernetes, которые достаточны для его работы
          («минимальный RBAC»).
        </li>
        <li>
          Компоненты не запускаются под root-правами. Исключения явно прописаны в списке разрешений.
        </li>
        <li>
          Корневая файловая система открыта только на чтение, за исключением отдельных директорий.
        </li>
        <li>
          Ни один компонент Deckhouse не открывает локальный порт без TLS-шифрования и аутентификации.
        </li>
        <li>
          Дополнительные запросы к API Kubernetes для проверки аутентификации и авторизации кешируются и не влияют на
          производительность.
        </li>
      </ul>
    </div>
  </div>

  <div class="features__item features__item_odd">
    <div class="features__item-content container">
      <div class="features__item-header">
        <h2 class="features__item-title text_h1">
          Авторизация<br>
          <small>упрощенный RBAC</small>
        </h2>
      </div>
      <ul class="features__item-list">
        <li>
          Линтеры проверяют, что RBAC-права описаны в определенном файле каждого модуля Deckhouse, явно и однозначно. Это обеспечивает
          единую точку контроля.
        </li>
        <li>
          Названия для Service Accounts, Roles, RoleBindings и т. п. строго регламентированы — это защищает от
          человеческих ошибок.
        </li>
        <li>
          Аутентификация между компонентами кластера всегда проводится одним из двух способов: через TLS или с помощью
          bearer-токенов. Авторизация — через механизмы Kubernetes (SubjectAccessReview).
        </li>
      </ul>
    </div>
  </div>
</section>

<section class="block container">
  <div class="block__content">
    <p class="text text_big">
      <strong>Пример:</strong> для мониторинга в кластере используется Prometheus. Он собирает данные со всех компонентов.
      У каждого из компонентов есть порт для подключения сервиса мониторинга. При подключении к этому порту Prometheus
      использует индивидуальный SSL-сертификат.
    </p>
    <p class="text text_big">
      Когда от Prometheus поступает запрос, компонент проводит аутентификацию — проверяет, что сертификат Prometheus
      подписан Certificate Authority Kubernetes; после этого — авторизацию, запрашивая SubjectAccessReview. Такой механизм
      гарантирует, что только Prometheus может подключаться к портам мониторинга.
    </p>
  </div>
</section>

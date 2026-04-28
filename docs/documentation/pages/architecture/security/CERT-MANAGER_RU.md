---
title: Модуль cert-manager
permalink: ru/architecture/security/cert-manager.html
lang: ru
search: cert-manager, сертификат, letsencrypt, acme
description: Архитектура модуля cert-manager в Deckhouse Kubernetes Platform.
---

Модуль [`cert-manager`](/modules/cert-manager/) автоматизирует полный цикл управления сертификатами в кластере: от выпуска и продления самоподписанных сертификатов до интеграции с внешними центрами сертификации, такими как Let's Encrypt, HashiCorp Vault и Venafi. Это существенно упрощает обеспечение безопасности сервисов и позволяет централизованно контролировать все процессы, связанные с сертификатами.

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

- На схеме контейнеры разных подов показаны как взаимодействующие напрямую. Фактически обмен выполняется через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса приводится над стрелкой.
- Поды могут быть запущены в нескольких репликах, однако на схеме каждый под показан в единственном экземпляре.
{% endalert %}

Архитектура модуля [`cert-manager`](/modules/cert-manager/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DKP изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля cert-manager](../../../images/architecture/security/c4-l2-cert-manager.ru.png)

## Компоненты модуля

Модуль `cert-manager` состоит из следующих компонентов:

1. **Cert-manager** — контроллер, обеспечивающий полный цикл управления сертификатами в Deckhouse Kubernetes Platform (DKP). **Cert-manager** управляет следующими кастомными ресурсами:

    - Issuer — описывает настройки и параметры для получения сертификатов от конкретного источника (например, CA или внешний сервис). Применяется внутри выбранного namespace;
    - ClusterIssuer — аналог Issuer, но действует на весь кластер и доступен во всех namespace;
    - Certificate — определяет, какой сертификат требуется: указывает параметры, такие как субъект, срок действия, использованный Issuer/ClusterIssuer и дополнительные опции;
    - CertificateRequest — заявка на выпуск или продление сертификатов;
    - Challenge — описывает задание, используемое для прохождения валидации домена (например, HTTP-01, DNS-01 challenge для Let's Encrypt);
    - Order — объединяет связанные Challenge в последовательность для получения сертификата у ACME-сервера (например, Let's Encrypt).

    Компонент содержит следующие контейнеры:

    - **cert-manager** — основной контейнер;
    - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контейнера cert-manager.

    {% alert level="info" %}
    **Cert-manager** поддерживает интеграцию с внешними компонентами для работы с DNS-провайдерами, которые не имеют нативной поддержки в контроллере, с помощью [webhook](https://cert-manager.io/docs/configuration/acme/dns01/webhook/). Эти интеграционные компоненты являются отдельными решениями и не входят в стандартную комплектацию модуля, однако для их корректной работы их необходимо устанавливать в системный namespace `d8-cert-manager`. При внесении изменений в этот namespace рекомендуется всегда учитывать наличие подобных расширений для сохранения их работоспособности.
    {% endalert %}

1. **Webhook** — компонент состоит из одного контейнера **webhook**, который обеспечивает следующие действия:
    - валидацию кастомных ресурсов Issuer, ClusterIssuer, Certificate, CertificateRequest, Challenge, Order;
    - мутацию кастомных ресурсов CertificateRequest, при этом **webhook** добавляет информацию о пользователе, создавшем запрос на сертификат.

    В DKP валидация отключена для ресурсов в неймспейсе `d8-cert-manager`, а также для неймспейсов с меткой `cert-manager.io/disable-validation=true`.

1. **Cainjector** — дополнительный компонент, состоящий из одного контейнера [**cainjector**](https://cert-manager.io/docs/concepts/ca-injector/). **Cainjector** отвечает за автоматическую подстановку или обновление сертификатов корневого центра сертификации (CA) во все релевантные ресурсы Kubernetes: ValidatingWebhookConfiguration, MutatingWebhookConfiguration, CustomResourceDefinition и APIService. Это обеспечивает актуальность доверенных корневых сертификатов для сервисов, использующих webhooks и расширения API.

    Активация **cainjector** возможна через параметр `.spec.settings.enableCAInjector` в [ModuleConfig модуля `cert-manager`](/modules/cert-manager/configuration.html).

    **Cainjector** обрабатывает только ресурсы с аннотациями  `cert-manager.io/inject-ca-from`, `cert-manager.io/inject-ca-from-secret` или `cert-manager.io/inject-apiserver-ca` в зависимости от типа ресурса.

1. **Cm-acme-http-solver** — временный pod с контейнером **acmesolver**, запускаемый для прохождения [HTTP-01 Challenge](https://cert-manager.io/docs/configuration/acme/http01/) при валидации домена через ACME (например, Let's Encrypt). Этот компонент автоматически создаётся **cert-manager** на время исполнения HTTP-01 Challenge и удаляется по завершении процедуры. Такой подход реализует безопасную временную публикацию ресурса, подтверждающего владение доменом для получения сертификата.

## Взаимодействия модуля

Модуль `cert-manager` взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

    - управляет кастомными ресурсами Issuer, ClusterIssuer, Certificate, CertificateRequest, Challenge, Order;
    - следит и обновляет ресурсы ValidatingWebhookConfiguration, MutatingWebhookConfiguration, CustomResourceDefinition и APIService.

1. **ACME-сервис** — выполняет запросы для подтверждения домена и выпуска сертификатов.

1. **PKI-сервис** — выполняет запросы для выпуска и обновления (перевыпуска) сертификатов.

1. **DNS-провайдер** — выполняет запросы на добавление и удаление записей в службах DNS для прохождения DNS-01 Challenge при валидации домена через ACME.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver**:
    - валидация кастомных ресурсов Issuer, ClusterIssuer, Certificate, CertificateRequest, Challenge, Order;
    - мутацию кастомных ресурсов CertificateRequest.

1. **Prometheus-main** — сбор метрик **cert-manager**.

1. **Nginx Controller** — пересылка запросов от ACME-сервисов к **cm-acme-http-solver**.

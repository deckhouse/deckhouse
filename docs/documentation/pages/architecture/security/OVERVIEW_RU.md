---
title: Подсистема Security
permalink: ru/architecture/security/
lang: ru
search: security, безопасность, подсистема безопасности
description: Архитектура подсистемы Security в Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

В данном подразделе описывается архитектура подсистемы Security (подсистемы безопасности) Deckhouse Kubernetes Platform (DKP).

{% alert level="info" %}
Практические материалы по этим модулям подробно разобраны в курсе [«Инструменты безопасности в Deckhouse Kubernetes Platform»](https://deckhouse.ru/courses/security-tools-in-dkp/) в [Deckhouse Академии](https://deckhouse.ru/academy/).
{% endalert %}

В подсистему Security входят следующие модули:

* [`admission-policy-engine`](/modules/admission-policy-engine/) — позволяет использовать в кластере политики безопасности согласно Kubernetes [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/). Для реализации политик модуль использует [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/);
* [`runtime-audit-engine`](/modules/runtime-audit-engine/) — реализует аудит безопасности рантайма (обнаружение угроз на основе событий ядра Linux и аудита Kubernetes API с помощью [Falco](https://falco.org/)). Архитектура описана в разделе [«Аудит безопасности рантайма»](runtime-audit.html);
* [`security-events-manager`](/modules/security-events-manager/) — выполняет декларативный сбор, обработку, нормализацию и доставку событий безопасности из логов приложений и инфраструктурных компонентов Kubernetes. Архитектура описана в разделе [«События безопасности»](security-events.html);
* [`operator-trivy`](/modules/operator-trivy/) — позволяет периодически сканировать кластер DKP на наличие уязвимостей;
* [`cert-manager`](/modules/cert-manager/) — управляет TLS-сертификатами в кластере;
* [`secrets-store-integration`](/modules/secrets-store-integration/) — реализует доставку секретов в приложения Kubernetes-кластера путем подключения секретов, ключей и сертификатов из внешних хранилищ;
* [`secret-copier`](/modules/secret-copier/) — автоматически копирует секреты в неймспейсы кластера.

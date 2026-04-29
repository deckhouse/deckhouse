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

В подсистему Security входят следующие модули:

* [`admission-policy-engine`](/modules/admission-policy-engine/) — позволяет использовать в кластере политики безопасности согласно Kubernetes [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/). Для реализации политик модуль использует [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/);
* [`runtime-audit-engine`](/modules/runtime-audit-engine/) — реализует внутреннюю систему обнаружения угроз;
* [`operator-trivy`](/modules/operator-trivy/) — позволяет периодически сканировать кластер DKP на наличие уязвимостей;
* [`cert-manager`](/modules/cert-manager/) — управляет TLS-сертификатами в кластере;
* [`secrets-store-integration`](/modules/secrets-store-integration/) — реализует доставку секретов в приложения Kubernetes-кластера путем подключения секретов, ключей и сертификатов из внешних хранилищ;
* [`secret-copier`](/modules/secret-copier/) — автоматически копирует секреты в неймспейсы кластера.

В подразделе на данный момент описаны следующие компоненты подсистемы Security:

* [контроль целостности](integrity-control.html);
* [аудит событий безопасности](runtime-audit.html).

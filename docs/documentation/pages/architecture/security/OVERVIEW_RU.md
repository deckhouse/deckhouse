---
title: Обзор
permalink: ru/architecture/security/
lang: ru
search: security, безопасность, подсистема безопасности
---

В данном подразделе описывается архитектура подсистемы **Security** (подсистемы безопасности) DKP.

В подсистему **Security** входят следующие модули:

* [admission-policy-engine](/modules/admission-policy-engine/) - Позволяет использовать в кластере политики безопасности согласно [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) Kubernetes. Модуль для работы использует [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/).
* [runtime-audit-engine](/modules/runtime-audit-engine/stable/) - реализует внутреннюю систему обнаружения угроз.
* [operator-trivy](modules/operator-trivy/stable/) - позволяет выполнять периодическое сканирование на уязвимости в кластере DKP.
* [cert-manager](/modules/cert-manager/) - управляет TLS-сертификатами в кластере.
* [secrets-store-integration](/modules/secrets-store-integration/stable/) - реализует доставку секретов для приложения в Kubernetes-кластерах путем подключения секретов, ключей и сертификатов, хранящихся во внешних хранилищах секретов.
* [secret-copier](/modules/secret-copier/) - автоматически копирует все секреты во все пространства имён кластера.

В подразделе на данный момент описаны:

* [контроль целостности](integrity-control.html)
* [аудит событий безопасности](runtime-audit.html)

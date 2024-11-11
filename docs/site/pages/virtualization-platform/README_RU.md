---
title: "Deckhouse Virtualization Platform"
permalink: ru/virtualization-platform/documentation/readme.html
lang: ru
---

Вы находитесь на главной странице документации Deckhouse Virtualization Platform.

Deckhouse Virtualization Platform (далее платформа) предназначена для запуска, управления и оркестрации виртуальных машин и связанных с ними ресурсами. Платформа основывается на платформе Deckhouse Kubernetes Platform, но предназначается для запуска виртуальных машин.



В тестовых целях

Управление ресурсами

{% alert level="warning" %}
Если вы планируете использовать Deckhouse Virtualization Platform в production-среде, рекомендуется разворачивать его на физических серверах. Развертывание Deckhouse Virtualization Platform на виртуальных машинах также возможно, но в этом случае необходимо включить nested-виртуализацию.
{% endalert %}

Для работы виртуализации требуется кластер Deckhouse Kubernetes Platform. Пользователям редакции Enterprise Edition доступна возможность управления ресурсами через графический интерфейс (UI).

Для подключения к виртуальным машинам с использованием последовательного порта, VNC или по протоколу ssh используется утилита командной строки [d8](https://deckhouse.ru/documentation/v1/deckhouse-cli/).

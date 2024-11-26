---
title: Настройка ПО безопасности для работы с Deckhouse
permalink: ru/security_software_setup.html
lang: ru
---

Если узлы кластера Kubernetes анализируются сканерами безопасности (антивирусными средствами), то может потребоваться их настройка для исключения ложноположительных срабатываний.

Deckhouse Kubernetes Platform (DKP) использует следующие директории при работе ([скачать в csv](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

## ПО безопасности

### KESL

Далее приведены рекомендации по настройке Kaspersky Endpoint Security for Linux (KESL) для обеспечения корректной работы с платформой Deckhouse Kubernetes Platform, независимо от выбранной редакции.

Для обеспечения совместимости с DKP на стороне KESL необходимо отключить следующие задачи:

- `Firewall_Management (ID: 12)`.
- `Web Threat Protection (ID: 14)`.
- `Network Threat protection (ID: 17)`.
- `Web Control (ID: 26)`.

{% alert level="info" %}
Список задач может отличаться в будущих версиях KESL.
{% endalert %}

Убедитесь, что узлы Kubernetes соответствуют минимальным требованиям к ресурсам, указанным для [DKP](https://deckhouse.ru/products/kubernetes-platform/guides/production.html#требования-к-ресурсам) и [KESL](https://support.kaspersky.com/KES4Linux/12.1.0/ru-RU/197642.htm).

При совместной эксплуатации KESL и Deckhouse может потребоваться оптимизация производительности согласно [рекомендациям Kaspersky](https://support.kaspersky.com/KES4Linux/12.1.0/ru-RU/206054.htm).

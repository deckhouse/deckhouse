---
title: KESL
permalink: ru/security/kesl.html
lang: ru
---

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

Убедитесь, что узлы Kubernetes соответствуют минимальным требованиям к ресурсам, указанным для DKP и [KESL](https://support.kaspersky.com/KES4Linux/12.1.0/ru-RU/197642.htm).

При совместной эксплуатации KESL и DKP может потребоваться оптимизация производительности согласно [рекомендациям Kaspersky](https://support.kaspersky.com/KES4Linux/12.1.0/ru-RU/206054.htm).

---
title: Настройка ПО безопасности для работы с Deckhouse
permalink: ru/security_software_setup.html
lang: ru
---

Если узлы кластера Kubernetes анализируются сканерами безопасности (антивирусными средствами), то может потребоваться их настройка, для исключения ложноположительных срабатываний.

Deckhouse использует следующие директории при работе ([скачать в csv...](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

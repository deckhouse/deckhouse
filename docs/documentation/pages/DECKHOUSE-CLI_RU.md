---
title: Deckhouse CLI
permalink: ru/deckhouse-cli/
description: Deckhouse CLI — интерфейс командной строки для работы с кластерами от разработчиков Deckhouse.
lang: ru
---

Deckhouse CLI — это интерфейс командной строки для работы с кластерами от Deckhouse Kubernetes Platform (DKP). Начиная с релиза 1.59, интерфейс автоматически устанавливается на все узлы кластера. Утилиту можно также [установить](#как-установить-deckhouse-cli) на любую машину и использовать для работы с кластерами без DKP.

В командной строке к утилите можно обратиться как `d8`. Все команды сгруппированы по функциям:

{% alert level="info" %}
Группы команд `d8 d` и `d8 mirror` недоступны для Community Edition (CE) и Basic Edition (BE).
{% endalert %}

* `d8 k` — команды, которые в кластерах Kubernetes выполняет `kubectl`.  
    Например, в кластере можно выполнить `kubectl get pods` как `d8 k get pods`.
* `d8 d` — команды, отвечающие за доставку по аналогии с утилитой `werf`.  
    Например, вместо `werf plan --repo registry.deckhouse.io` можно выполнить `d8 d plan --repo registry.deckhouse.io`.

* `d8 mirror` — команды, которые позволяют скопировать образы дистрибутива DKP в частный container registry (ранее для этого использовалась утилита `dhctl mirror`).
  Например, можно выполнить `d8 mirror pull -l <LICENSE> <TAR-BUNDLE-PATH>` вместо `dhctl mirror --license <LICENSE> --images-bundle-path <TAR-BUNDLE-PATH>`.

  Сценарии использования:

  - [ручная загрузка образов в изолированный приватный registry](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#%D1%80%D1%83%D1%87%D0%BD%D0%B0%D1%8F-%D0%B7%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B0-%D0%BE%D0%B1%D1%80%D0%B0%D0%B7%D0%BE%D0%B2-%D0%B2-%D0%B8%D0%B7%D0%BE%D0%BB%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%BD%D1%8B%D0%B9-%D0%BF%D1%80%D0%B8%D0%B2%D0%B0%D1%82%D0%BD%D1%8B%D0%B9-registry);
  - [ручная загрузка образов подключаемых модулей Deckhouse в изолированный приватный registry](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#%D1%80%D1%83%D1%87%D0%BD%D0%B0%D1%8F-%D0%B7%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B0-%D0%BE%D0%B1%D1%80%D0%B0%D0%B7%D0%BE%D0%B2-%D0%BF%D0%BE%D0%B4%D0%BA%D0%BB%D1%8E%D1%87%D0%B0%D0%B5%D0%BC%D1%8B%D1%85-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D0%B5%D0%B9-deckhouse-%D0%B2-%D0%B8%D0%B7%D0%BE%D0%BB%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%BD%D1%8B%D0%B9-%D0%BF%D1%80%D0%B8%D0%B2%D0%B0%D1%82%D0%BD%D1%8B%D0%B9-registry).

* `d8 v` — команды, отвечающие за работу с виртуальными машинами, созданными [Deckhouse Virtualization Platform](/modules/virtualization/stable/).  
    Например, команда `d8 virtualization console` подключает к консоли виртуальной машины.

    <div markdown="0">
    <details><summary>Больше команд для виртуализации...</summary>
    <ul>
    <li><code>d8 v console</code> подключает к консоли виртуальной машины.</li>
    <li><code>d8 v port-forward</code> перенаправляет локальные порты на виртуальную машину.</li>
    <li><code>d8 v scp</code> использует клиент SCP для работы с файлами на виртуальной машине.</li>
    <li><code>d8 v ssh</code> подключает к виртуальной машине по SSH.</li>
    <li><code>d8 v vnc</code> подключает к виртуальной машине по VNC.</li>
    </ul>
    </details>
    </div>

## Как установить Deckhouse CLI?

{% include d8-cli-install/main.liquid %}

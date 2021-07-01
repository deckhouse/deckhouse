---
title: "Быстрый старт"
permalink: ru/getting_started.html
layout: page-nosidebar
lang: ru
toc: false
---

{::options parse_block_html="false" /}

<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>

<div markdown="1">
Скорее всего вы уже ознакомились с основными [возможностями Deckhouse Platform](/ru/#features). В данном руководстве рассмотрен пошаговый процесс установки платформы.

Установка платформы Deckhouse возможна как на железные серверы (bare metal), так и в инфраструктуру одного из поддерживаемых облачных провайдеров. В зависимости от выбранной инфраструктуры процесс может немного отличаться, поэтому ниже приведены примеры установки для разных вариантов.

## Установка

### Требования и подготовка

Установка Deckhouse в общем случае выглядит так:

-  На локальной машине (с которой будет производиться установка) запускается Docker-контейнер.
-  Этому контейнеру передаются приватный SSH-ключ с локальной машины и файл конфигурации будущего кластера в формате YAML (например, `config.yml`).
-  Контейнер подключается по SSH к целевой машине (для bare metal-инсталляций) или облаку, после чего происходит непосредственно установка и настройка кластера Kubernetes.

***Примечание**: при установке Deckhouse в публичное облако для Kubernetes-кластера будут использоваться «обычные» вычислительные ресурсы провайдера, а не managed-решение с Kubernetes, предлагаемое провайдером.*

Ограничения/требования для установки:

-   На машине, с которой будет производиться установка, необходимо наличие Docker runtime.
-   Deckhouse поддерживает разные версии Kubernetes: с 1.16 по 1.21 включительно. Однако обратите внимание, что для установки «с чистого листа» доступны только версии 1.16, 1.19, 1.20 и 1.21. В примерах конфигурации ниже используется версия 1.19.
-   Рекомендованная минимальная аппаратная конфигурация для будущего кластера:
    -   не менее 4 ядер CPU;
    -   не менее 8  ГБ RAM;
    -   не менее 40 ГБ дискового пространства для кластера и данных etcd;
    -   ОС: Ubuntu Linux 16.04/18.04/20.04 LTS или CentOS 7;
    -   доступ к интернету и стандартным репозиториям используемой ОС для установки дополнительных необходимых пакетов.

## Шаг 1. Конфигурация

Выберите тип инфраструктуры, в которую будет устанавливаться Deckhouse:
</div>

<div class="tabs">
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure active"
  onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_bm');">
    Bare Metal
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_yc');">
    Yandex.Cloud
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_aws');">
    Amazon AWS
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_gcp');">
    Google Cloud
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_azure');">
    Microsoft Azure
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_openstack');">
    OpenStack
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_existing');">
    Существующий кластер
  </a>
</div>

<div id="infrastructure_bm" class="tabs__content tabs__content_infrastructure active" markdown="1">
{% include getting_started/STEP1_BAREMETAL_RU.md %}

{% include getting_started/STEP2_RU.md mode="baremetal" %}

{% include getting_started/STEP3_RU.md mode="baremetal" %}
</div>

<div id="infrastructure_yc" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_CLOUD_YANDEX_RU.md %}

{% include getting_started/STEP2_RU.md mode="cloud" %}

{% include getting_started/STEP3_RU.md mode="cloud" %}
</div>

<div id="infrastructure_aws" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_CLOUD_AWS_RU.md %}

{% include getting_started/STEP2_RU.md mode="cloud" %}

{% include getting_started/STEP3_RU.md mode="cloud" %}
</div>

<div id="infrastructure_gcp" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_CLOUD_GCP_RU.md %}

{% include getting_started/STEP2_RU.md mode="cloud" %}

{% include getting_started/STEP3_RU.md mode="cloud" %}
</div>

<div id="infrastructure_openstack" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_CLOUD_OPENSTACK_RU.md %}

{% include getting_started/STEP2_RU.md mode="cloud" provider="openstack" %}

{% include getting_started/STEP3_RU.md mode="cloud" %}
</div>

<div id="infrastructure_azure" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_CLOUD_AZURE_RU.md %}

{% include getting_started/STEP2_RU.md mode="cloud" provider="azure" %}

{% include getting_started/STEP3_RU.md mode="cloud" %}
</div>

<div id="infrastructure_existing" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_EXISTING_RU.md %}

{% include getting_started/STEP2_RU.md mode="existing" %}

{% include getting_started/STEP3_RU.md mode="existing" %}
</div>

<div markdown="1">
## Следующие шаги

### Работа с модулями

Модульная система Deckhouse позволяет «на лету» добавлять и убирать модули из кластера. Для этого необходимо отредактировать конфигурацию кластера — все изменения применятся автоматически.

Например, добавим модуль [user-authn](/ru/documentation/v1/modules/150-user-authn/):

- Открываем конфигурацию Deckhouse:
  ```shell
kubectl -n d8-system edit cm/deckhouse
```
- Находим секцию `data` и включаем в ней модуль:
  ```yaml
  data:
    userAuthnEnabled: "true"
  ```
- Сохраняем конфигурацию. В этот момент Deckhouse понимает, что произошли изменения, и модуль устанавливается автоматически.

Для изменения настроек модуля необходимо повторить пункт 1, т.е. внести изменения в конфигурацию и сохранить их. Изменения автоматически применятся.

Для отключения модуля потребуется аналогичным образом задать параметру значение `false`.

### Куда двигаться дальше?

Все установлено, настроено и работает. Подробная информация о системе в целом и по каждому компоненту Deckhouse расположена в [документации](/ru/documentation/v1/).

По всем возникающим вопросам связывайтесь с нашим [онлайн-сообществом](/ru/community/about.html#online-community).
</div>

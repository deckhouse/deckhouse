---
title: "Модуль registry: примеры"
description: ""
---

## Переключение на режим `Direct`

Для переключения уже работающего кластера на режим `Direct` необходимо выполнить следующие шаги:

1. Убедитесь, что модуль `registry` включен и работает.

```bash
kubectl get module registry
```

<!-- markdownlint-disable MD029 -->
2. Добавьте следующие настройки в `ModuleConfig` модуля `deckhouse`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    mode: Direct
    direct:
      host: registry.deckhouse.ru/deckhouse/ee
      scheme: https
      license: <LICENSE_KEY> # Замените на ваш лицензионный ключ
```

{% alert level="warning" %}
Если у вас используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/deckhouse/) для корректной настройки.
{% endalert %}

<!-- markdownlint-disable MD029 -->
3. Проверьте статус переключения registry в секрете `registry-state` используя [инструкцию](./faq.html#how-to-check-the-registry-mode-switch-status).

<!--
Ниже приведены примеры запуска и отключения модуля `registry`

## Bootstrap кластера с Proxy режимом

- Подготовьте конфигурационные файлы для bootstrap-а нового кластера
- Добавьте в `config.yml` манифест [InitConfiguration](/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration) с указанием использования `Proxy` режима. Пример:

  ```yaml
  apiVersion: deckhouse.io/v2alpha1
  kind: InitConfiguration
  deckhouse:
  registry:
    mode: Proxy
    proxy:
      imagesRepo: nexus.company.my/deckhouse/ee
      username: "nexus-user"
      password: "nexus-p@ssw0rd"
      scheme: HTTPS
      ca: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
  ```

  Где:
  - `registry.mode` - выбранный режим registry
  - `registry.proxy` - параметры для режима proxy (если `registry.mode: Proxy`). Подробнее в разделе [настройка](/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration-registry-proxy)
- Выполните bootstrap кластера

## Bootstrap кластера с Mirror режимом

- Создайте `d8.tar` архив с запакованными docker образами используя утилиту `d8 mirror pull`, аналогично документации:
  - [ручная загрузка образов в изолированный приватный registry](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#%D1%80%D1%83%D1%87%D0%BD%D0%B0%D1%8F-%D0%B7%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B0-%D0%BE%D0%B1%D1%80%D0%B0%D0%B7%D0%BE%D0%B2-%D0%B2-%D0%B8%D0%B7%D0%BE%D0%BB%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%BD%D1%8B%D0%B9-%D0%BF%D1%80%D0%B8%D0%B2%D0%B0%D1%82%D0%BD%D1%8B%D0%B9-registry);
  - [ручная загрузка образов подключаемых модулей Deckhouse в изолированный приватный registry](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#%D1%80%D1%83%D1%87%D0%BD%D0%B0%D1%8F-%D0%B7%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B0-%D0%BE%D0%B1%D1%80%D0%B0%D0%B7%D0%BE%D0%B2-%D0%BF%D0%BE%D0%B4%D0%BA%D0%BB%D1%8E%D1%87%D0%B0%D0%B5%D0%BC%D1%8B%D1%85-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D0%B5%D0%B9-deckhouse-%D0%B2-%D0%B8%D0%B7%D0%BE%D0%BB%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%BD%D1%8B%D0%B9-%D0%BF%D1%80%D0%B8%D0%B2%D0%B0%D1%82%D0%BD%D1%8B%D0%B9-registry).
  
  Пример:

  ```bash
  d8 mirror pull \
    --source='registry.deckhouse.ru/deckhouse/ee' \
    --license='<LICENSE_KEY>' '<--release=X.Y.Z or --min-version=X.Y>' $(pwd)/d8.tar
  ```

- Подготовьте конфигурационные файлы для bootstrap-а нового кластера
- Добавьте в `config.yml` манифест [InitConfiguration](/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration) с указанием использования `Mirror`. Пример:

  ```yaml
  apiVersion: deckhouse.io/v2alpha1
  kind: InitConfiguration
  deckhouse:
  registry:
    mode: Detached
    detached:
      imagesBundlePath: ~/deckhouse/d8.tar
  ```

  Где:
  - `registry.mode` - выбранный режим registry
  - `registry.detached` - параметры для режима detached (если `registry.mode: Detached`). Подробнее в разделе [настройка](/products/kubernetes-platform/documentation/v1/installing/configuration.html#initconfiguration-registry-detached)
- Выполните bootstrap кластера. Во время шага `051_bootstrap_system_registry_img_push` будет выполнен автоматический пуш образов в `registry`

## Запуск Proxy режима на запущенном кластере

- Запустите модуль registry. Пример:

  ```bash
  kubectl apply -f - <<EOF
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: registry
  spec:
    version: 1
    enabled: true
    settings:
      mode: Proxy
      proxy:
        host: registry.deckhouse.ru
        scheme: https
        path: /deckhouse/ee
        password: "password"
        user: "user"
  EOF
  ```

  Где:
  - `settings.mode` - выбранный режим registry
  - `settings.proxy` - параметры для режима proxy (если `registry.mode: Proxy`). Подробнее в разделе [настройка](./configuration.html)
- Дождитесь применения конфигурации для `containerd` через bashible
- Выполните переключение на новый адрес docker registry. Для подключения используйте адрес:
  - ?????????????????????

## Запуск Mirror режима на запущенном кластере

- Запустите модуль registry. Пример:

  ```bash
  kubectl apply -f - <<EOF
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: registry
  spec:
    version: 1
    enabled: true
    settings:
      mode: Detached
  EOF
  ```

  Где:
  - `settings.mode` - выбранный режим registry
  - `settings.detached` - параметры для режима detached (если `registry.mode: Detached`). Подробнее в разделе [настройка](./configuration.html)

- **TODO**:
- Дождитесь применения конфигурации для `containerd` через bashible
- Выполните переключение на новый адрес docker registry. Для подключения используйте адрес: ...

## Выключение модуля

- **TODO**:
- Переключение на другой registry
- Отключение модуля registry
-->

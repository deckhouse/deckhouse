---
title: "Настройка containerd"
permalink: ru/stronghold/documentation/admin/platform-management/node-management/containerd.html
lang: ru
---

## Общие сведения

Дополнительная настройка containerd возможна через создание конфигурационных файлов с помощью ресурса NodeGroupConfiguration.

За настройки containerd отвечает встроенный скрипт [`032_configure_containerd.sh`](https://github.com/deckhouse/deckhouse/blob/main/candi/bashible/common-steps/all/032_configure_containerd.sh.tpl) — он производит объединение всех конфигурационных файлов сервиса `containerd` расположенных по пути `/etc/containerd/conf.d/*.toml`, а также перезапуск сервиса.

При разработке `NodeGroupConfiguration` следует учитывать следующее:

1. Директория `/etc/containerd/conf.d/` не создается автоматически;
1. Создавать файлы в данной директории следует до запуска `032_configure_containerd.sh`, т.е. с приоритетом менее `32`.

## Дополнительные настройки containerd

{% alert level="critical" %}
Добавление кастомных настроек вызывает перезапуск сервиса `containerd`.
{% endalert %}

{% alert level="warning" %}
Вы можете переопределять значения параметров, которые заданы в файле `/etc/containerd/deckhouse.toml`, но при этом ответственность за их корректную работу ляжет на вас. Рекомендуется избегать внесения изменений, которые могут повлиять на master-узлы.
{% endalert %}

## Включение метрик для containerd

Простейший пример добавления настроек `containerd` — включение метрик.

Обратите внимание:
1. Скрипт создаёт директорию с конфигурационными файлами.
2. Скрипт создаёт файл в директории `/etc/containerd/conf.d`.
3. Скрипт имеет приоритет 31 (`weight: 31`).
4. Конфигурация на мастер-узлах не изменяется, только на узлах группы `worker`.
5. Сбор метрик нужно будет конфигурировать отдельно, это только их включение.
6. Скрипт использует конструкцию bashbooster [bb-sync-file](http://www.bashbooster.net/#sync) для синхронизации содержимого файла.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-enable-metrics.sh
spec:
  bundles:
    - '*'
  content: |
    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/metrics_settings.toml - << EOF
    [metrics]
    address = "127.0.0.1"
    grpc_histogram = true
    EOF
  nodeGroups:
    - "worker"
  weight: 31
```

## Добавление приватного registry с авторизацией

Для запуска собственных приложений может потребоваться приватный registry, доступ к которому может быть закрыт авторизацией. `containerd` позволяет задать настройки registry через  параметр `plugins."io.containerd.grpc.v1.cri".registry`.

Данные для авторизации указываются в параметре `auth` в формате docker registry auth виде base64 строки. Строку можно получить такой командой:

```shell
d8 k create secret docker-registry my-secret --dry-run=client --docker-username=User --docker-password=password --docker-server=private.registry.example -o jsonpath="{ .data['\.dockerconfigjson'] }"
eyJhdXRocyI6eyJwcml2YXRlLnJlZ2lzdHJ5LmV4YW1wbGUiOnsidXNlcm5hbWUiOiJVc2VyIiwicGFzc3dvcmQiOiJwYXNzd29yZCIsImF1dGgiOiJWWE5sY2pwd1lYTnpkMjl5WkE9PSJ9fX0=
```

Ресурс NodeGroupConfiguration выглядит так:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config.sh
spec:
  bundles:
    - '*'
  content: |

    REGISTRY_URL=private.registry.example

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
          endpoint = ["https://${REGISTRY_URL}"]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
          auth = "eyJhdXRocyI6eyJwcml2YXRlLnJlZ2lzdHJ5LmV4YW1wbGUiOnsidXNlcm5hbWUiOiJVc2VyIiwicGFzc3dvcmQiOiJwYXNzd29yZCIsImF1dGgiOiJWWE5sY2pwd1lYTnpkMjl5WkE9PSJ9fX0="
    EOF
  nodeGroups:
    - "*"
  weight: 31
```

## Добавление сертификата для дополнительного registry

<span id="ca-сертификат-для-дополнительного-registry"></span>

Приватный registry может требовать корневого сертификата, его нужно добавить в директорию `/var/lib/containerd/certs` и указать в параметре tls в настройках containerd.

За основу такого скрипта можно взять [инструкцию](os.html#добавление-корневого-сертификата) по добавлению корневого сертификата в ОС. Обратите внимание на отличия:

1. Значение приоритета 31;
2. Корневой сертификат добавляется в директорию `/var/lib/containerd/certs`;
3. Путь к сертификату добавляется в секцию настроек `plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls`.

Скрипт использует конструкции bashbooster:

- [bb-sync-file](http://www.bashbooster.net/#sync) для синхронизации содержимого файла.
- [bb-tmp-file](http://www.bashbooster.net/#tmp) для создания временных файлов и их удаления после выполнения скрипта.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: configure-cert-containerd.sh
spec:
  bundles:
  - '*'
  nodeGroups:
  - '*'
  weight: 31
  content: |-
    REGISTRY_URL=private.registry.example
    CERT_FILE_NAME=${REGISTRY_URL}
    CERTS_FOLDER="/var/lib/containerd/certs/"
    CERT_CONTENT=$(cat <<"EOF"
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIRAJ4RR/WDuAym7M11JA8W7D0wDQYJKoZIhvcNAQELBQAw
    JTEjMCEGA1UEAxMabmV4dXMuNTEuMjUwLjQxLjIuc3NsaXAuaW8wHhcNMjQwODAx
    MTAzMjA4WhcNMjQxMDMwMTAzMjA4WjAlMSMwIQYDVQQDExpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAL1p
    WLPr2c4SZX/i4IS59Ly1USPjRE21G4pMYewUjkSXnYv7hUkHvbNL/P9dmGBm2Jsl
    WFlRZbzCv7+5/J+9mPVL2TdTbWuAcTUyaG5GZ/1w64AmAWxqGMFx4eyD1zo9eSmN
    G2jis8VofL9dWDfUYhRzJ90qKxgK6k7tfhL0pv7IHDbqf28fCEnkvxsA98lGkq3H
    fUfvHV6Oi8pcyPZ/c8ayIf4+JOnf7oW/TgWqI7x6R1CkdzwepJ8oU7PGc0ySUWaP
    G5bH3ofBavL0bNEsyScz4TFCJ9b4aO5GFAOmgjFMMUi9qXDH72sBSrgi08Dxmimg
    Hfs198SZr3br5GTJoAkCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB
    /wQCMAAwUwYDVR0RBEwwSoIPbmV4dXMuc3ZjLmxvY2FsghpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pb4IbZG9ja2VyLjUxLjI1MC40MS4yLnNzbGlwLmlvMA0GCSqG
    SIb3DQEBCwUAA4IBAQBvTjTTXWeWtfaUDrcp1YW1pKgZ7lTb27f3QCxukXpbC+wL
    dcb4EP/vDf+UqCogKl6rCEA0i23Dtn85KAE9PQZFfI5hLulptdOgUhO3Udluoy36
    D4WvUoCfgPgx12FrdanQBBja+oDsT1QeOpKwQJuwjpZcGfB2YZqhO0UcJpC8kxtU
    by3uoxJoveHPRlbM2+ACPBPlHu/yH7st24sr1CodJHNt6P8ugIBAZxi3/Hq0wj4K
    aaQzdGXeFckWaxIny7F1M3cIWEXWzhAFnoTgrwlklf7N7VWHPIvlIh1EYASsVYKn
    iATq8C7qhUOGsknDh3QSpOJeJmpcBwln11/9BGRP
    -----END CERTIFICATE-----
    EOF
    )

    CONFIG_CONTENT=$(cat <<EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
        ca_file = "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"
    EOF
    )

    mkdir -p ${CERTS_FOLDER}
    mkdir -p /etc/containerd/conf.d


    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"

    CONFIG_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CONFIG_CONTENT}" > "${CONFIG_TMP_FILE}"

    # Ensure CA certificate file in the CERTS_FOLDER.
    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE}

    # Ensure additional containerd configuration file.
    bb-sync-file \
      "/etc/containerd/conf.d/${REGISTRY_URL}.toml" \
      ${CONFIG_TMP_FILE}
```

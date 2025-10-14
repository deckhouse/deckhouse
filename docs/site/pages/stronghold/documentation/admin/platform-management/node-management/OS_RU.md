---
title: "Настройка ОС"
permalink: ru/stronghold/documentation/admin/platform-management/node-management/os.html
lang: ru
---

## Установка плагина cert-manager для kubectl на master-узлах

NodeGroupConfiguration можно использовать для установки нужных утилит на мастер-узлы.

Например, можно установить утилиту cmctl от проекта cert-manager. Эту команду также можно использовать как kubectl plugin.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: kubectl-plugin-cert-manager.sh
spec:
  weight: 100
  bundles:
    - "*"
  nodeGroups:
    - "master"
  content: |
    # See https://github.com/cert-manager/cmctl/releases/tag/v2.1.0
    version=v2.1.1

    if [ -x /usr/local/bin/kubectl-cert_manager ]; then
      exit 0
    fi
    curl -L https://github.com/cert-manager/cmctl/releases/download/${version}/cmctl_linux_amd64.tar.gz -o - | tar zxf - cmctl
    mv cmctl /usr/local/bin
    ln -s /usr/local/bin/cmctl /usr/local/bin/kubectl-cert_manager
```

## Задание параметра sysctl

Для некоторых задач на узлах нужно изменять параметры sysctl.

Например, приложения, использующие mmapfs, могут потребовать увеличения разрешённого процессу количества отображений в память. Это количество устанавливается параметром `vm.max_map_count` и может быть задано через NodeGroupConfiguration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: sysctl-tune.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "worker"
  content: |
    sysctl -w vm.max_map_count=262144
```

## Установка нужной версии ядра

Узлы могут требовать определённой версии ядра Linux и NodeGroupConfiguration может помочь в этом случае. Для упрощения скрипта лучше использовать конструкции [bashbooster](http://www.bashbooster.net/).

Разные ОС требуют разных операций при смене версии ядра, поэтому далее приведены примеры для Debian и CentOS.

В обоих примерах используется конструкция `bb-deckhouse-get-disruptive-update-approval` — расширение набора команд bashbooster от Deckhouse. Эта конструкция предотвращает перезагрузку узла, если требуется подтверждение перезагрузки путём добавления аннотации на узел.

Помимо этого, используются следующие конструкции bashbooster:

- [bb-apt-install](http://www.bashbooster.net/#apt) для установки apt пакета и отправки события "bb-package-installed", если пакет был установлен;
- [bb-yum-install](http://www.bashbooster.net/#yum) для установки apt пакета и отправки события "bb-package-installed", если пакет был установлен;
- [bb-event-on](http://www.bashbooster.net/#event) для сигнализации, что нужна перезагрузка узла, если отправлено событие "bb-package-installed";
- [bb-log-info](http://www.bashbooster.net/#log) для логирования;
- [bb-flag-set](http://www.bashbooster.net/#flag) для сигнализации, что нужен перезапуск узла.

### Для дистрибутивов, основанных на Debian

Создайте ресурс NodeGroupConfiguration, указав в переменной `desired_version` желаемую версию ядра:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    desired_version="5.15.0-53-generic"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-apt-install "linux-image-${desired_version}"
```

### Для дистрибутивов, основанных на CentOS

Создайте ресурс NodeGroupConfiguration, указав в переменной `desired_version` желаемую версию ядра:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    desired_version="3.10.0-1160.42.2.el7.x86_64"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-yum-install "kernel-${desired_version}"
```

## Добавление корневого сертификата

<span id="добавление-ca-сертификата"></span>

В некоторых случаях может потребоваться дополнительный корневой сертификат, например, для доступа к внутренним ресурсам организации. Добавление корневых сертификатов можно оформить в виде NodeGroupConfiguration.

{% alert level="warning" %}
Данный пример приведен для ОС Ubuntu.  
Способ обновления хранилища сертификатов может отличаться в зависимости от ОС.

При адаптации скрипта под другую ОС измените параметр [`bundles`](/modules/node-manager/cr.html#nodegroupconfiguration-v1alpha1-spec-bundles).
{% endalert %}

Скрипт использует конструкции bashbooster:

- [bb-sync-file](http://www.bashbooster.net/#sync) для синхронизации содержимого файла и отправки события "ca-file-updated, если файл изменился;
- [bb-event-on](http://www.bashbooster.net/#event) для запуска обновления сертификатов, если отправлено событие "ca-file-updated";
- [bb-tmp-file](http://www.bashbooster.net/#tmp) для создания временных файлов и их удаления после выполнения скрипта.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-custom-ca.sh
spec:
  weight: 31
  nodeGroups:
  - '*'  
  bundles:
  - 'ubuntu-lts'
  content: |-
    CERT_FILE_NAME=example_ca
    CERTS_FOLDER="/usr/local/share/ca-certificates"
    CERT_CONTENT=$(cat <<EOF
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

    bb-event-on "ca-file-updated" "update-certs"
    
    update-certs() {          # Function with commands for adding a certificate to the store
      update-ca-certificates
    }

    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} \
      ca-file-updated   
```

Аналогично настраивается корневой сертификат для containerd, пример приведён в разделе [настроек containerd](containerd.html#добавление-сертификата-для-дополнительного-registry).

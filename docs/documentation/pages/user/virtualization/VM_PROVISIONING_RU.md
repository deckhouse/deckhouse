---
title: "Начальная инициализация и агент гостевой ОС"
permalink: ru/user/virtualization/vm-provisioning.html
description: "Начальная настройка гостевой системы через cloud-init и Sysprep, а также агент гостевой ОС и что он даёт виртуальной машине."
search: cloud-init, Sysprep, provisioning, агент гостевой ОС, qemu-guest-agent
lang: ru
---

При первом запуске гостевая система настраивается сценарием начальной инициализации, а дальше модуль общается с ней через агента гостевой ОС.

## Сценарии начальной инициализации ВМ

Сценарии начальной инициализации предназначены для первичной конфигурации виртуальной машины при её запуске.

В качестве сценариев начальной инициализации поддерживаются:

- [Cloud-Init](https://cloudinit.readthedocs.io).
- [Sysprep](https://learn.microsoft.com/ru-ru/windows-hardware/manufacture/desktop/sysprep--system-preparation--overview).

### Cloud-Init

Cloud-Init — это инструмент для автоматической настройки виртуальных машин при первом запуске. Он позволяет выполнять широкий спектр задач конфигурации без ручного вмешательства.

{% alert level="warning" %}
Конфигурация Cloud-Init записывается в формате YAML и должна начинаться с заголовка `#cloud-config` в начале блока конфигурации. О других возможных заголовках и их назначении вы можете узнать в [официальной документации по cloud-init](https://cloudinit.readthedocs.io/en/latest/explanation/format.html#headers-and-content-types).
{% endalert %}

Основные возможности Cloud-Init:

- создание пользователей, установка паролей, добавление SSH-ключей для доступа;
- автоматическая установка необходимого программного обеспечения при первом запуске;
- запуск произвольных команд и скриптов для настройки системы;
- автоматический запуск и включение системных сервисов (например, [`qemu-guest-agent`](#агент-гостевой-ос)).

Ниже приведены типичные сценарии.

1. Добавление SSH-ключа для [предустановленного пользователя](../../admin/configuration/virtualization/cluster-images.html#image-resources-table), который уже может присутствовать в cloud-образе (например, пользователь `ubuntu` в официальных образах Ubuntu). Имя такого пользователя зависит от образа. Уточните его в документации к вашему дистрибутиву.

   ```yaml
   #cloud-config
   ssh_authorized_keys:
     - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD... your-public-key ...
   ```

1. Создание пользователя с паролем и SSH-ключом:

   ```yaml
   #cloud-config
   users:
     - name: cloud
       passwd: <PASSWORD_HASH>
       lock_passwd: false
       sudo: ALL=(ALL) NOPASSWD:ALL
       shell: /bin/bash
       ssh-authorized-keys:
         - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD... your-public-key ...
   ssh_pwauth: True
   ```

   Здесь `<PASSWORD_HASH>` — хеш пароля в кавычках, полученный командой `mkpasswd --method=SHA-512 --rounds=4096`.

1. Установка пакетов и сервисов:

   ```yaml
   #cloud-config
   package_update: true
   packages:
     - nginx
     - qemu-guest-agent
   runcmd:
     - systemctl daemon-reload
     - systemctl enable --now nginx.service
     - systemctl enable --now qemu-guest-agent.service
   ```

Ниже показано, как передать сценарий виртуальной машине:

{% tabs cloud-init-usage %}

{% tab "В командной строке" %}

Сценарий Cloud-Init можно встраивать непосредственно в спецификацию виртуальной машины, но этот сценарий ограничен максимальной длиной в 2048 байт:

```yaml
spec:
  provisioning:
    type: UserData
    userData: |
      #cloud-config
      package_update: true
      ...
```

Если сценарий длинный или содержит приватные данные, создайте его в ресурсе Secret. Пример ресурса Secret со сценарием Cloud-Init приведён ниже:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloud-init-example
data:
  userData: <base64 data>
type: provisioning.virtualization.deckhouse.io/cloud-init
```

Фрагмент конфигурации виртуальной машины при использовании скрипта начальной инициализации Cloud-Init, хранящегося в ресурсе Secret:

```yaml
spec:
  provisioning:
    type: UserDataRef
    userDataRef:
      kind: Secret
      name: cloud-init-example
```

> Значение поля `.data.userData` должно быть закодировано в формате Base64. Для кодирования можно использовать команду `base64 -w 0` или `echo -n "content" | base64`.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
1. Создайте виртуальную машину или выберите существующую и нажмите на её имя.
1. На вкладке «Конфигурация» прокрутите страницу вниз до переключателя «Cloud-init» и включите его.
1. Выберите режим заполнения:
   - «Базовая настройка» — заполните поля «Имя пользователя», «Пароль» и «Публичный SSH-ключ», при необходимости включите переключатель «Неограниченный sudo-доступ». Конфигурацию cloud-init платформа сформирует сама;
   - «Редактирование» — введите конфигурацию cloud-init вручную в поле «Параметры». Под полем отображается использованный объём (не более 2048 байт). В поле «Связанный секрет» можно выбрать существующий скрипт инициализации, и его содержимое загрузится в поле. Если секрет не привязан, конфигурация хранится в спецификации ВМ.
1. Нажмите появившуюся кнопку «Сохранить» (при создании ВМ — кнопку «Создать»).

Сценарий можно хранить отдельным ресурсом и переиспользовать для нескольких ВМ. Чтобы создать такой ресурс:

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Скрипты инициализации».
1. Нажмите кнопку «Создать».
1. В поле «Имя» введите имя скрипта, в поле «Тип» выберите `cloud-init` или `sysprep`.
1. В блоке «Файлы» в поле «Имя файла» задайте ключ (по умолчанию — `userData`), а содержимое введите вручную, перетащите файл в поле или нажмите на него, чтобы загрузить файл.
1. Нажмите кнопку «Создать».

В разделе «Скрипты инициализации» отображаются секреты с типом `provisioning.virtualization.deckhouse.io/*`, для каждого показаны имя, тип (`cloud-init` или `sysprep`), список ключей и возраст ресурса. Чтобы виртуальная машина использовала такой скрипт, сошлитесь на него в параметре [`.spec.provisioning.userDataRef`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-provisioning-userdataref).

{% endtab %}

{% endtabs %}

### Sysprep

Для конфигурирования виртуальных машин под управлением ОС Windows с использованием Sysprep поддерживается только вариант с ресурсом Secret.

Пример ресурса Secret со сценарием Sysprep:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sysprep-example
data:
  unattend.xml: <base64 data>
type: provisioning.virtualization.deckhouse.io/sysprep
```

{% alert level="info" %}
Значение поля `.data.unattend.xml` должно быть закодировано в формате Base64. Для кодирования можно использовать команду `base64 -w 0` или `echo -n "content" | base64`.
{% endalert %}

Фрагмент конфигурации виртуальной машины с использованием скрипта начальной инициализации Sysprep в ресурсе Secret:

```yaml
spec:
  provisioning:
    type: SysprepRef
    sysprepRef:
      kind: Secret
      name: sysprep-example
```

## Агент гостевой ОС

Установите в гостевую систему QEMU Guest Agent, чтобы модуль мог взаимодействовать с операционной системой внутри ВМ. Агент нужен для трёх вещей:

- он позволяет создавать консистентные снимки дисков и ВМ;
- он сообщает сведения о работающей системе, и они попадают в блок [`.status.guestOSInfo`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-status-guestosinfo);
- по нему видно, что операционная система действительно загрузилась, а не просто запустилась виртуальная машина.

Модуль работает с `qemu-guest-agent` версии 5.2.0 и выше. Проверить установленную версию можно командой:

```bash
qemu-guest-agent --version
```

Сведения о гостевой системе выглядят так:

```yaml
status:
  guestOSInfo:
    id: fedora
    kernelRelease: 6.11.4-301.fc41.x86_64
    kernelVersion: "#1 SMP PREEMPT_DYNAMIC Sun Oct 20 15:02:33 UTC 2024"
    machine: x86_64
    name: Fedora Linux
    prettyName: Fedora Linux 41 (Cloud Edition)
    version: 41 (Cloud Edition)
    versionId: "41"
```

Работает ли агент, показывает колонка `AGENT`:

```bash
d8 k get vm -o wide
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME     PHASE     UPTIME   CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS    AGE
fedora   Running   5d21h    6       5%             8000Mi   False          True    True         virtlab-pt-1   10.66.10.1   5d21h
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Установите агента командой для вашего дистрибутива и запустите службу:

```bash
# Debian и производные.
sudo apt install qemu-guest-agent

# CentOS и производные.
sudo yum install qemu-guest-agent

sudo systemctl enable --now qemu-guest-agent
```

Для Linux установку удобно автоматизировать сценарием начальной инициализации:

```yaml
#cloud-config
package_update: true
packages:
  - qemu-guest-agent
runcmd:
  - systemctl enable --now qemu-guest-agent.service
```

Настраивать агента после установки не требуется. Если снимкам нужна согласованность данных приложения, положите скрипты подготовки в каталог `/etc/qemu-ga/hooks.d/` на Debian и Ubuntu либо `/etc/qemu/fsfreeze-hook.d/` на RHEL, CentOS и Fedora. Скрипты должны быть исполняемыми, агент запускает их до заморозки файловой системы и после её разморозки, поэтому сервисы приложения останавливать не приходится.

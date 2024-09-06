---
title: Настройка ПО безопасности для работы с Deckhouse
permalink: ru/security_software_setup.html
lang: ru
---

Если узлы кластера Kubernetes анализируются сканерами безопасности (антивирусными средствами), то может потребоваться их настройка для исключения ложноположительных срабатываний.

Deckhouse использует следующие директории при работе ([скачать в csv...](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

## Рекомендации по настройке KESL (Kaspersky Endpoint Security для Linux) для работы совместно с Deckhouse

Чтобы KESL не влиял на работоспособность Deckhouse, необходимо выполнить описанные ниже рекомендации по настройке через командную строку непосредственно на хосте или через систему централизованного удалённого управления Kaspersky Security Center.

Управление KESL осуществляется [с помощью задач](https://support.kaspersky.com/KES4Linux/11.2.0/ru-RU/199339.htm), имеющих определённые номера. Ниже рассмотрена общая настройка, и конфигурация основных задач, используемых при настройке KESL в Deckhouse.

### Общая настройка

Измените настройки KESL относительно битов маркировки сетевых пакетов, так как существуют пересечения с маркировкой сетевых пакетов самим Deckhouse. Для этого:
1. Остановите KESL, если он запущен, и измените настройки:

   * в параметре `BypassFwMark` измените значение с `0x400` на `0x700`;
   * в параметре `NtpFwMark` измените значение с `0x200` на `0x600`.

1. Запустите KESL.

   Пример команд для запуска KESL:

   ```shell
   systemctl stop kesl
   sed -i "s/NtpFwMark=0x200/NtpFwMark=0x600/" /var/opt/kaspersky/kesl/common/kesl.ini
   sed -i "s/BypassFwMark=0x400/BypassFwMark=0x700/" /var/opt/kaspersky/kesl/common/kesl.ini
   systemctl start kesl
   ```

### Задача №1 «Защита от файловых угроз» (File_Threat_Protection)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 1 --add-exclusion /etc/cni
kesl-control --set-settings 1 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 1 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 1 --add-exclusion /mnt/vector-data
kesl-control --set-settings 1 --add-exclusion /opt/cni/bin
kesl-control --set-settings 1 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 1 --add-exclusion /var/lib/bashable
kesl-control --set-settings 1 --add-exclusion /var/lib/containerd
kesl-control --set-settings 1 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 1 --add-exclusion /var/lib/etcd
kesl-control --set-settings 1 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 1 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 1 --add-exclusion /var/log/containers
kesl-control --set-settings 1 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

### Задача №2 «Поиск вредоносного ПО» (Scan_My_Computer)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 2 --add-exclusion /etc/cni
kesl-control --set-settings 2 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 2 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 2 --add-exclusion /mnt/vector-data
kesl-control --set-settings 2 --add-exclusion /opt/cni/bin
kesl-control --set-settings 2 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 2 --add-exclusion /var/lib/bashable
kesl-control --set-settings 2 --add-exclusion /var/lib/containerd
kesl-control --set-settings 2 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 2 --add-exclusion /var/lib/etcd
kesl-control --set-settings 2 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 2 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 2 --add-exclusion /var/log/containers
kesl-control --set-settings 2 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

### Задача №3 «Выборочная проверка» (Scan_File)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 3 --add-exclusion /etc/cni
kesl-control --set-settings 3 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 3 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 3 --add-exclusion /mnt/vector-data
kesl-control --set-settings 3 --add-exclusion /opt/cni/bin
kesl-control --set-settings 3 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 3 --add-exclusion /var/lib/bashable
kesl-control --set-settings 3 --add-exclusion /var/lib/containerd
kesl-control --set-settings 3 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 3 --add-exclusion /var/lib/etcd
kesl-control --set-settings 3 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 3 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 3 --add-exclusion /var/log/containers
kesl-control --set-settings 3 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

### Задача №4 «Проверка важных областей» (Critical_Areas_Scan)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 4 --add-exclusion /etc/cni
kesl-control --set-settings 4 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 4 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 4 --add-exclusion /mnt/vector-data
kesl-control --set-settings 4 --add-exclusion /opt/cni/bin
kesl-control --set-settings 4 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 4 --add-exclusion /var/lib/bashable
kesl-control --set-settings 4 --add-exclusion /var/lib/containerd
kesl-control --set-settings 4 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 4 --add-exclusion /var/lib/etcd
kesl-control --set-settings 4 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 4 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 4 --add-exclusion /var/log/containers
kesl-control --set-settings 4 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

### Задача №11 «Контроль целостности системы» (System_Integrity_Monitoring)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 11 --add-exclusion /etc/cni
kesl-control --set-settings 11 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 11 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 11 --add-exclusion /mnt/vector-data
kesl-control --set-settings 11 --add-exclusion /opt/cni/bin
kesl-control --set-settings 11 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 11 --add-exclusion /var/lib/bashable
kesl-control --set-settings 11 --add-exclusion /var/lib/containerd
kesl-control --set-settings 11 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 11 --add-exclusion /var/lib/etcd
kesl-control --set-settings 11 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 11 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 11 --add-exclusion /var/log/containers
kesl-control --set-settings 11 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

### Задача №12 «Управление сетевым экраном» (Firewall_Management)

{% alert level="danger" %}
Задачу необходимо отключить и не включать. Включение задачи приведёт к неработоспособности Deckhouse.
{% endalert %}

Эта задача удаляет все правила iptables, не относящиеся к KESL ([ссылка на документацию KESL](https://support.kaspersky.com/KES4Linux/11.3.0/ru-RU/234820.htm)).

Если задача включена, отключите её, выполнив команду:

```shell
kesl-control --stop-task 12
```

### Задача №13 «Защита от шифрования» (Anti_Cryptor)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 13 --add-exclusion /etc/cni
kesl-control --set-settings 13 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 13 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 13 --add-exclusion /mnt/vector-data
kesl-control --set-settings 13 --add-exclusion /opt/cni/bin
kesl-control --set-settings 13 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 13 --add-exclusion /var/lib/bashable
kesl-control --set-settings 13 --add-exclusion /var/lib/containerd
kesl-control --set-settings 13 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 13 --add-exclusion /var/lib/etcd
kesl-control --set-settings 13 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 13 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 13 --add-exclusion /var/log/containers
kesl-control --set-settings 13 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

### Задача №14 «Защита от веб-угроз» (Web_Threat_Protection)

Задачу рекомендуется отключить.

Если есть необходимость включить задачу, выполните настройку таким образом, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 14
```

### Задача №17 «Защита от сетевых угроз» (Network_Threat_Protection)

Задачу рекомендуется отключить.

Если есть необходимость включить задачу, выполните настройку таким образом, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 17
```

### Задача №20 «Анализ поведения» (Behavior_Detection)

С настройками по умолчанию задача негативного влияния на работоспособность Deckhouse не оказывает.

Если есть необходимость включить задачу, выполните настройку таким образом, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 20
```

### Задача №21 «Контроль программ» (Application_Control)

С настройками по умолчанию задача негативного влияния на работоспособность Deckhouse не оказывает.

Если есть необходимость включить задачу, выполните настройку таким образом, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 21
```

### Задача №22 «Веб-контроль» (Web_Control)

Задачу рекомендуется отключить.

Если есть необходимость включить задачу, выполните настройку таким образом, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 22
```

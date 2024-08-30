---
title: Настройка ПО безопасности для работы с Deckhouse
permalink: ru/security_software_setup.html
lang: ru
---

Если узлы кластера Kubernetes анализируются сканерами безопасности (антивирусными средствами), то может потребоваться их настройка, для исключения ложноположительных срабатываний.

Deckhouse использует следующие директории при работе ([скачать в csv...](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

## Рекомендации по настройке KESL (Kaspersky Endpoint Security для Linux) для работы совместно с Deckhouse

Для того чтобы KESL не влиял на работоспособность Deckhouse, необходимо выполнить следующие рекомендации по настройке через командную строку непосредственно на хосте или через систему централизованного удаленного управления Kaspersky Security Center.

### Общая настройка

Измените настройки KESL относительно битов маркировки сетевых пакетов, так как существуют пересечения с маркировкой сетевых пакетов самим Deckhouse:

* в параметре BypassFwMark измените значение с 0x400 на 0x700
* в параметре NtpFwMark измените значение с 0x200 на 0x600

Для этого сначала остановите KESL, если он запущен, измените настройки и затем запустите KESL.

Пример команд:

```shell
systemctl stop kesl
sed -i "s/NtpFwMark=0x200/NtpFwMark=0x600/" /var/opt/kaspersky/kesl/common/kesl.ini
sed -i "s/BypassFwMark=0x400/BypassFwMark=0x700/" /var/opt/kaspersky/kesl/common/kesl.ini
systemctl start kesl
```

### Задача №1 «Защита от файловых угроз» (File_Threat_Protection)

Добавьте в исключение директории Deckhouse, выполнив команды:

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

Примечание: при добавлении может показать уведомление, что некоторых директорий не существует, но правило будет добавлено – это нормально.

### Задача №2 «Поиск вредоносного ПО» (Scan_My_Computer)

Добавьте в исключение директории Deckhouse, выполнив команды:

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

Примечание: при добавлении может показать уведомление, что некоторых директорий не существует, но правило будет добавлено – это нормально.

### Задача №3 «Выборочная проверка» (Scan_File)

Добавьте в исключение директории Deckhouse, выполнив команды:

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

Примечание: при добавлении может показать уведомление, что некоторых директорий не существует, но правило будет добавлено – это нормально.

### Задача №4 «Проверка важных областей» (Critical_Areas_Scan)

Добавьте в исключение директории Deckhouse, выполнив команды:

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

Примечание: при добавлении может показать уведомление, что некоторых директорий не существует, но правило будет добавлено – это нормально.

### Задача №11 «Контроль целостности системы» (System_Integrity_Monitoring)

Добавьте в исключение директории Deckhouse, выполнив команды:

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

Примечание: при добавлении может показать уведомление, что некоторых директорий не существует, но правило будет добавлено – это нормально.

### Задача №12 «Управление сетевым экраном» (Firewall_Management)

Задачу **НЕОБХОДИМО ОТКЛЮЧИТЬ И НЕ ВКЛЮЧАТЬ**!!! Потому что это приведет к неработоспособности Deckhouse. Дело в том, что данная задача удаляет все правила iptables, не относящиеся к KESL ([ссылка на документацию KESL](https://support.kaspersky.com/KES4Linux/11.3.0/ru-RU/234820.htm)).

Если задача включена, то отключите её, выполнив команду:

```shell
kesl-control --stop-task 12
```

### Задача №13 «Защита от шифрования» (Anti_Cryptor)

Добавьте в исключение директории Deckhouse, выполнив команды команды:

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

Примечание: при добавлении может показать уведомление, что
некоторых директорий не существует, но правило будет
добавлено – это нормально.

### Задача №14 «Защита от веб-угроз» (Web_Threat_Protection)

Рекомендуем отключить, но если есть необходимость включить задачу, то настраивайте самостоятельно, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, то отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 14
```

### Задача №17 «Защита от сетевых угроз» (Network_Threat_Protection)

Рекомендуем отключить, но если есть необходимость включить задачу, то настраивайте самостоятельно, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, то отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 17
```

### Задача №20 «Анализ поведения» (Behavior_Detection)

С настройками по умолчанию негативного влияния на работоспособность Deckhouse не оказывает. Если есть необходимость включить задачу, то настраивайте самостоятельно, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, то отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 20
```

### Задача №21 «Контроль программ» (Application_Control)

С настройками по умолчанию негативного влияния на работоспособность Deckhouse не оказывает. Если есть необходимость включить задачу, то настраивайте самостоятельно, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, то отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 21
```

### Задача №22 «Веб-Контроль» (Web_Control)

Рекомендуем отключить, но если есть необходимость включить задачу, то настраивайте самостоятельно, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, то отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 22
```

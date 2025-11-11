---
title: "Общее управление и конфигурация control plane"
permalink: ru/admin/configuration/platform-scaling/control-plane/control-plane-management-and-configuration.html
description: "Настройка и управление Kubernetes control plane в Deckhouse Kubernetes Platform. Высокая доступность, управление сертификатами и конфигурация компонентов control plane."
lang: ru
---

## Основные возможности

Deckhouse Kubernetes Platform (DKP) управляет компонентами управляющего слоя кластера (control plane) с помощью модуля [`control-plane-manager`](/modules/control-plane-manager/). Этот модуль запускается на всех управляющих узлах (master-узлах) с лейблом `node-role.kubernetes.io/control-plane: ""`.

Функционал управления control plane включает:

- Управление сертификатами необходимых для работы control plane, в том числе их продление и выпуск при изменении конфигурации. Автоматически поддерживается безопасная конфигурация и возможность быстрого добавления дополнительных SAN для организации защищённого доступа к API Kubernetes.

- Настройка компонентов. DKP генерирует все необходимые конфигурации и манифесты (kube-apiserver, etcd и др.), снижая вероятность ручных ошибок.

- Upgrade/downgrade компонентов. DKP поддерживает согласованное обновление или понижение версии control plane, что позволяет поддерживать единообразие версий в кластере.

- Управление конфигурацией etcd-кластера и его членов. DKP масштабирует master-узлы, выполняет миграцию из single-master в multi-master и обратно.

- Настройка kubeconfig — DKP формирует актуальный конфигурационный файл (с правами `cluster-admin`), автоматическое продление и обновление, а также создание `symlink` для пользователя `root`.

> Некоторые параметры, влияющие на работу control plane, берутся из ресурса [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration).

## Включение, отключение и настройка модуля

### Включение / отключение

Включить или выключить модуль [`control-plane-manager`](/modules/control-plane-manager/) можно следующими способами:

1. Создайте (или измените) ресурс ModuleConfig/control-plane-manager, указав `spec.enabled: true` или `false`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     enabled: true
   ```

1. Используйте команду:

   ```bash
   d8 system module enable control-plane-manager
   ```

   или

   ```bash
   d8 system module disable control-plane-manager
   ```

1. Через [веб-интерфейс Deckhouse](/modules/console/):

   - перейдите в раздел «Deckhouse — «Модули»;
   - найдите модуль `control-plane-manager` и нажмите на него;
   - включите тумблер «Модуль включен».

### Настройка

Чтобы настроить модуль, используйте ModuleConfig/control-plane-manager и укажите необходимые параметры в [`spec.settings`](/modules/control-plane-manager/configuration.html#parameters-settings).

Пример с указанием версии схемы, включённым модулем и несколькими настройками:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  enabled: true
  settings:
    apiserver:
      bindToWildcard: true
      certSANs:
      - bakery.infra
      - devs.infra
      loadBalancer: {}
```

### Проверка состояния и очередей DKP

Как проверить, что [`control-plane-manager`](/modules/control-plane-manager/) корректно запущен и не находится в состоянии ожидания, а также как проверить активные задачи (очереди) в DKP:

1. Убедитесь, что модуль включён:

   ```shell
   d8 k get modules control-plane-manager
   ```

1. Проверьте состояние подов `control-plane-manager` (поды находятся в пространстве имён `kube-system` и имеют лейбл `app=d8-control-plane-manager`):

   ```shell
   d8 k -n kube-system get pods -l app=d8-control-plane-manager -o wide
   ```

   Убедитесь, что все поды находятся в статусе `Running` (или `Completed`).

1. Проверьте, что master-узлы в состоянии `Ready`:

   ```shell
   d8 k get nodes -l node-role.kubernetes.io/control-plane
   ```

   Если требуется посмотреть более подробную информацию:

   ```shell
   d8 k describe node <имя-узла>
   ```

1. Получите список очередей и активных заданий:

   ```shell
   d8 system queue list
   ```

   Пример вывода:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

{% alert level="warning" %}
Перед выполнением «тяжёлых» процедур (например, перед переводом кластера из single-master в multi-master или перед обновлением версии Kubernetes) рекомендуется дождаться, чтобы все задачи в очередях были выполнены.
{% endalert %}

## Управление сертификатами

В DKP за выпуск и продление всех SSL-сертификатов компонентов control plane отвечает модуль [`control-plane-manager`](/modules/control-plane-manager/). Он контролирует:

1. **Серверные сертификаты** для kube-apiserver и etcd, хранящиеся в секрете `d8-pki` (пространство имён `kube-system`):
   - Корневой CA Kubernetes (`ca.crt`, `ca.key`);
   - Корневой CA etcd (`etcd/ca.crt`, `etcd/ca.key`);
   - RSA-сертификат и ключ для подписи Service Account’ов (`sa.pub`, `sa.key`);
   - Корневой CA для extension API-серверов (`front-proxy-ca.key`, `front-proxy-ca.crt`).

1. **Клиентские сертификаты**, необходимые для взаимного подключения компонентов control plane (например, `apiserver.crt`, `apiserver-etcd-client.crt` и т. д.). Эти файлы хранятся только на узлах. При любом изменении (например, добавлении новых SAN) сертификаты автоматически перевыпускаются, а kubeconfig синхронизируется.

### Управление PKI

DKP также управляет инфраструктурой приватных ключей (PKI), которая необходима для шифрования и аутентификации во всём кластере Kubernetes:

- PKI для компонентов control plane (kube-apiserver, kube-controller-manager, kube-scheduler и т. д.).
- PKI для кластера etcd (сертификаты etcd и межузлового взаимодействия).

DKP «забирает» управление этими PKI после завершения первоначальной установки кластера и запуска своих подов. Таким образом, все операции по выпуску, продлению и обновлению ключей (как для control plane, так и для etcd) выполняются автоматически и централизованно, без необходимости ручного вмешательства.

### Дополнительные SAN и автоматическое обновление

Deckhouse упрощает добавление новых точек входа (SAN) для API Kubernetes: достаточно прописать их в конфигурации. После любого изменения в SAN модуль автоматически регенерирует сертификаты и обновляет kubeconfig.

Чтобы добавить дополнительные SAN (дополнительные DNS-имена или IP-адреса) для API Kubernetes пропишите новые SAN в [параметре `spec.settings.apiserver.certSANs`](/modules/control-plane-manager/configuration.html#parameters-apiserver-certsans) вашего ресурса ModuleConfig/control-plane-manager.

DKP автоматически сгенерирует новые сертификаты и обновит все необходимые конфигурационные файлы (включая kubeconfig).

### Ротация сертификатов kubelet

В Deckhouse Kubernetes Platform ротация сертификатов kubelet происходит автоматически.
Параметры `--tls-cert-file` и `--tls-private-key-file` для kubelet не задаются напрямую. Вместо этого используется механизм динамических TLS-сертификатов: kubelet применяет клиентский сертификат по пути `/var/lib/kubelet/pki/kubelet-client-current.pem`, с помощью которого он запрашивает у kube-apiserver новый клиентский или серверный сертификат (файл `/var/lib/kubelet/pki/kubelet-server-current.pem`).

Из-за использования динамических TLS механизмов, [модуль `operator-trivy`](/modules/operator-trivy/) исключает проверки CIS Benchmark AVD-KCV-0088 и AVD-KCV-0089, так как они предполагают явную передачу параметров `--tls-cert-file` и `--tls-private-key-file` в конфигурации kubelet.

Особенности ротации сертификатов kubelet в Deckhouse Kubernetes Platform:

- По умолчанию kubelet генерирует собственные ключи в каталоге `/var/lib/kubelet/pki/` и при необходимости самостоятельно запрашивает продление сертификатов у kube-apiserver.
- Срок действия выданных сертификатов составляет 1 год (8760 часов). Когда до истечения срока действия остаётся от 5 до 10% времени (точное значение выбирается случайным образом из этого диапазона), kubelet автоматически инициирует запрос на новый сертификат. Подробнее см. в [официальной документации Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#bootstrap-initialization). При необходимости срок действия сертификатов можно изменить с помощью параметра `--cluster-signing-duration` в манифесте `/etc/kubernetes/manifests/kube-controller-manager.yaml`. Однако, чтобы kubelet успел получить и установить новый сертификат до истечения текущего, рекомендуется устанавливать срок действия сертификатов не менее чем на 1 час.
- Если kubelet не успеет обновить сертификат до его истечения, он утратит возможность выполнять запросы к kube-apiserver и, соответственно, продлевать сертификаты. В результате узел будет помечен как `NotReady` и автоматически пересоздан.

### Ручное обновление сертификатов компонентов control plane

Если master-узлы долго не были доступны (к примеру, серверы были выключены), возможна ситуация, когда некоторые сертификаты в управляющем слое утрачивают актуальность. После включения узлов автоматического обновления не произойдёт — нужно выполнить процедуру вручную.

Обновление сертификатов компонентов управляющего слоя происходит с помощью утилиты `kubeadm`.
Чтобы обновить сертификаты, выполните следующие действия на каждом master-узле:

1. Найдите утилиту `kubeadm` на master-узле и создайте символьную ссылку c помощью следующей команды:

   ```shell
   ln -s  $(find /var/lib/containerd  -name kubeadm -type f -executable -print) /usr/bin/kubeadm
   ```

1. Выполните команду:

   ```shell
   kubeadm certs renew all
   ```

   Она пересоздаст нужные сертификаты (kube-apiserver, kube-controller-manager, kube-scheduler, etcd и т.д.).

## Ускорение перезапуска подов при потере связи с узлом

По умолчанию, если узел в течении 40 секунд не сообщает свое состояние, он помечается как недоступный (`Unreachable`). И еще через 5 минут поды узла начнут перезапускаться на других узлах. В итоге общее время недоступности приложений составляет около 6 минут.

В специфических случаях, когда приложение не может быть запущено в нескольких экземплярах, есть способ сократить период их недоступности:

1. Уменьшить время перехода узла в состояние `Unreachable` при потере с ним связи настройкой [параметра `nodeMonitorGracePeriodSeconds`](/modules/control-plane-manager/configuration.html#parameters-nodemonitorgraceperiodseconds).
1. Установить меньший таймаут удаления подов с недоступного узла в [параметре `failedNodePodEvictionTimeoutSeconds`](/modules/control-plane-manager/configuration.html#parameters-failednodepodevictiontimeoutseconds).

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    nodeMonitorGracePeriodSeconds: 10
    failedNodePodEvictionTimeoutSeconds: 50
```

{% alert level="warning" %}
Чем короче таймауты, тем чаще системные компоненты проверяют состояние узлов и планируют перемещение подов. Это повышает нагрузку на control plane, поэтому выбирайте значения, соответствующие вашим требованиям к отказоустойчивости и производительности.
{% endalert %}

## Принудительное отключение IPv6 на узлах кластера

Внутреннее взаимодействие между компонентами кластера DKP осуществляется по протоколу IPv4. Однако, на уровне операционной системы узлов кластера, как правило, по умолчанию активен IPv6. Это приводит к автоматическому присвоению IPv6-адресов всем сетевым интерфейсам, включая интерфейсы подов. В результате возникает нежелательный сетевой трафик — например, избыточные DNS-запросы типа `AAAA`, которые могут повлиять на производительность и усложнить отладку сетевых взаимодействий.

Для корректного отключения IPv6 на уровне узлов в кластере, управляемом DKP, достаточно задать необходимые параметры через ресурс [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: disable-ipv6.sh
spec:
  nodeGroups:
  - '*'
  bundles:
  - '*'
  weight: 50
  content: |
    GRUB_FILE_PATH="/etc/default/grub"
    
    if ! grep -q "ipv6.disable" "$GRUB_FILE_PATH"; then
      sed -E -e 's/^(GRUB_CMDLINE_LINUX_DEFAULT="[^"]*)"/\1 ipv6.disable=1"/' -i "$GRUB_FILE_PATH"
      update-grub
      
      bb-flag-set reboot
    fi
```

{% alert level="warning" %}
После применения ресурса настройки GRUB будут обновлены, и узлы кластера начнут последовательную перезагрузку для применения изменений.
{% endalert %}

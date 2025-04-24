---
title: Хранилище, балансировка и особенности эксплуатации
permalink: ru/admin/integrations/public/yandex/yandex-storage.html
lang: ru
---

Этот раздел охватывает дополнительные аспекты интеграции Deckhouse с Yandex Cloud:

- Подключение облачных дисков через CSI;
- Автоматическое создание StorageClass;
- Использование балансировщиков нагрузки;
- Особенности применения изменений;
- Работу с CloudStatic-узлами, bastion-хостами и параметрами DHCP.

## Хранилище (CSI и StorageClass)

Модуль `cloud-provider-yandex` автоматически создаёт StorageClass под все поддерживаемые типы дисков Yandex Cloud:

| Тип диска                 | Имя StorageClass          | Комментарии              |
|--------------------------|---------------------------|--------------------------|
| `network-hdd`            | `network-hdd`             | —                        |
| `network-ssd`            | `network-ssd`             | —                        |
| `network-ssd-nonreplicated` | `network-ssd-nonreplicated` | Размер кратен 93 ГБ      |
| `network-ssd-io-m3`      | `network-ssd-io-m3`       | Размер кратен 93 ГБ      |

### Исключение ненужных StorageClass

Чтобы не создавать ненужные StorageClass, используйте параметр `exclude`:

```yaml
settings:
  storageClass:
    exclude:
    - network-ssd-.*
    - network-hdd
```

### Назначение StorageClass по умолчанию

Если требуется использовать конкретный StorageClass по умолчанию, задайте его в параметре `global.defaultClusterStorageClass`:

```console
kubectl edit mc global
```

## Балансировка нагрузки

### Внешний LoadBalancer

Deckhouse автоматически создаёт ресурсы NetworkLoadBalancer и TargetGroup в Yandex Cloud при создании Kubernetes-сервиса с типом `LoadBalancer`.

### Внутренний LoadBalancer

Чтобы создать внутренний балансировщик, добавьте аннотацию `yandex.cpi.flant.com/listener-subnet-id` в объект `Service`:

```yaml
metadata:
  annotations:
    yandex.cpi.flant.com/listener-subnet-id: <SubnetID>
```

## Особенности применения изменений

При изменении параметров модуля `cloud-provider-yandex` пересоздание уже существующих объектов `Machine` не выполняется.
Пересоздание происходит только при изменении параметров NodeGroup или YandexInstanceClass.

После внесения изменений в YandexClusterConfiguration необходимо выполнить:

```console
dhctl converge
```

## CloudStatic-узлы

Чтобы включить существующую ВМ в кластер как узел CloudStatic:

1. В метаданные виртуальной машины (через консоль Yandex Cloud) добавьте:

   ```yaml
   key: node-network-cidr
   value: <nodeNetworkCIDR из кластера>
   ```

1. Узнать значение `nodeNetworkCIDR` можно командой:

   ```console
   kubectl -n kube-system get secret d8-provider-cluster-configuration -o json | \
     jq --raw-output '.data."cloud-provider-cluster-configuration.yaml"' | base64 -d | grep '^nodeNetworkCIDR'
   ```

## Bastion-хост и bootstrap

Для доступа к узлам через bastion-хост:

1. Выполните bootstrap базовой инфраструктуры:

   ```console
   dhctl bootstrap-phase base-infra --config config.yml
   ```

1. Создайте bastion-хост:

   ```console
   yc compute instance create \
     --name bastion \
     --hostname bastion \
     --create-boot-disk image-family=ubuntu-2204-lts,image-folder-id=standard-images,size=20,type=network-hdd \
     --memory 2 \
     --cores 2 \
     --core-fraction 100 \
     --ssh-key ~/.ssh/id_rsa.pub \
     --zone ru-central1-a \
     --public-address 178.154.226.159
   ```

1. Продолжите установку:

   ```console
   dhctl bootstrap --ssh-bastion-host=178.154.226.159 --ssh-bastion-user=yc-user \
     --ssh-user=ubuntu --ssh-agent-private-keys=/tmp/.ssh/id_rsa --config=/config.yml
   ```

## DHCP и DNS (dhcpOptions)

Если используется секция `dhcpOptions`:

```yaml
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - 192.168.0.2
  - 192.168.0.3
```

Важно:

- Указанные DNS-серверы должны разрешать все необходимые зоны (внешние и внутренние).
- После изменения настроек потребуется:
  - выполнить `netplan apply` или аналог для обновления DHCP lease;
  - перезапустить все поды с `hostNetwork`, особенно `kube-dns`, чтобы обновился `resolv.conf`.

## Дополнительные внешние сети

DKP позволяет явно указать список дополнительных внешних сетей, IP-адреса из которых будут рассматриваться как External IP при создании и описании узлов.
Для этого используется параметр `settings.additionalExternalNetworkIDs` в ресурсе ModuleConfig модуля `cloud-provider-yandex`.

Этот параметр полезен, если:

- у вас есть дополнительные подсети с внешним доступом, не указанные явно в `externalSubnetIDs`;
- требуется точный контроль над тем, какие IP-адреса считаются публичными;
- нужно работать с кастомными схемами маршрутизации или подключениями через шлюзы NAT.

Если параметр `additionalExternalNetworkIDs` не задан, модуль сам определяет внешние сети только на основе настроек в YandexClusterConfiguration.

Пример конфигурации ModuleConfig с указанием внешних сетей:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  enabled: true
  settings:
    additionalExternalNetworkIDs:
      - enp6t4snovl2ko4p15em
```
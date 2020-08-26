---
title: "Модуль cloud-provider-yandex"
---

## Содержимое модуля

1. cloud-controller-manager — контроллер для управления ресурсами облака из Kubernetes.
    * Синхронизирует метаданные Yandex Instances и Kubernetes Nodes. Удаляет из Kubernetes ноды, которых более нет в Yandex.
2. CSI storage — для заказа дисков в Yandex.
3. Регистрация в модуле [node-manager]({{ site.baseurl }}/modules/040-node-manager/), чтобы [YandexInstanceClass'ы](#yandexinstanceclass-custom-resource) можно было использовать в [CloudInstanceClass'ах]({{ site.baseurl }}/modules/040-node-manager/#nodegroup-custom-resource).

## Конфигурация

### Включение модуля

Модуль по-умолчанию **выключен**. Для включения:

1. Корректно [настроить](#настройка-окружения) окружение.
2. Инициализировать deckhouse, передав параметр install.sh — `--extra-config-map-data base64_encoding_of_custom_config`.
3. Настроить параметры модуля.

### Параметры

> **Внимание!** При изменении конфигурационных параметров приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `NodeGroup` и `YandexInstanceClass`. См. подробнее в документации модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/#как-мне-перекатить-машины-с-новой-конфигурацией).

* `folderID` — имя каталога в Yandex, к которому будут привязаны compute ресурсы.
* `region` — имя региона, где будут заказываться инстансы.
* `zones` — Список зон из `region`, где будут заказываться instances. Является значением по-умолчанию для поля zones в [NodeGroup]({{ site.baseurl }}/modules/040-node-manager/#nodegroup-custom-resource) объекте.
  * Формат — массив строк.
* `zoneToSubnetIdMap` — карта для сопоставления zone и subnet
  * Формат — объект ключ-значение, где ключом является имя зоны, а значение - subnet, который относится к данной зоне
* `defaultLbListenerSubnetId` — Subnet ID, что будет использовать для создаваемового Listener'а в LoadBalancers.
  * Формат — строка.
  * **Внимание!** При указании данной опции все создаваемые LoadBalancer'ы будут INTERNAL типом. Для переопределения типа следует использовать аннотацию на Kubernetes Service с ключом `yandex.cpi.flant.com/loadbalancer-external` и любым значением. [Подробнее](#loadbalancer).
* `defaultLbTargetGroupNetworkId` — Network ID, что будет использовать для создаваемых Target Groups в LoadBalancers.
  * Формат — строка.
* `routeTableID` — Route Table ID, что будет использоваться для создания маршрутов к PodCIDR между виртуальными машинами. Должна быть привязана ко всему subnets, что используются на виртуальных машинах в кластере.
  * Формат — строка.
* `internalNetworkIDs` — список Network ID, которые будут считаться `InternalIP` при перечислении адресов у Node;
  * Формат — массив строк.
  * Опциональный параметр.
* `externalNetworkIDs` — список Network ID, которые будут считаться `ExternalIP` при перечислении адресов у Node;
  * Формат — массив строк.
  * Опциональный параметр.
* `sshUser` — пользователь для подключения по SSH.
* `sshKey` — публичный SSH ключ.
  * Формат — строка, как из `~/.ssh/id_rsa.pub`.
* `serviceAccountJSON` — авторизованный ключ для Service Account'у с правами editor для каталога.
  * Формат — строка c JSON.
  * [Как получить](https://cloud.yandex.ru/docs/iam/operations/iam-token/create-for-sa#via-cli).
* `dns` — параметры переопределения DNS параметров, получаемых по DHCP от Yandex.
* **Внимание!** Эта опция – workaround отсутствия возможности управления DNS в Яндекс. Как только такая возможность появится – опция станет deprectated.
  * `nameservers` — массив nameserver'ов, которые будут использоваться вместо получаемых по DHCP от Yandex.
    * Формат — массив строк. Например, `["1.1.1.1", "8.8.8.8"]`.
    * Опциональный параметр.
  * `search` — массив search доменов, которые будут использоваться вместо полученного search по DHCP от Yandex. Если передать пустой массив, то вообще не будет search доменов.
    * Формат — массив строк. Например `["example.com", "example.org"]`.
    * Опциональный параметр.
* `internalSubnet` — subnet CIDR, использующийся для внутренней межнодовой сети. Используется для настройки параметра `--iface-regex` во flannel.
  * Формат — string. Например, `10.201.0.0/16`.
  * Опциональный параметр.

#### Пример конфигурации

```yaml
cloudProviderYandexEnabled: "true"
cloudProviderYandex: |
  folderID: agsgfreafewqwqewqf2
  region: ru-central1
  zones:
  - ru-central1-a
  - ru-central1-b
  - ru-central1-c
  zoneToSubnetIdMap:
    ru-central1-a: zfdsafsadfdsafwr3422
    ru-central1-b: werqewr3241321sacasf
    ru-central1-c: weqfssgvfrwt42qr231d
  defaultLbTargetGroupNetworkId: safwqrefdwefewf13dsa
  sshUser:  ubuntu
  sshKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD5sAcceTHeT6ZnU+PUF1rhkIHG8/B36VWy/j7iwqqimC9CxgFTEi8MPPGNjf+vwZIepJU8cWGB/By1z1wLZW3H0HMRBhv83FhtRzOaXVVHw38ysYdQvYxPC0jrQlcsJmLi7Vm44KwA+LxdFbkj+oa9eT08nQaQD6n3Ll4+/8eipthZCDFmFgcL/IWy6DjumN0r4B+we+W4PIQ5Z4njrOzze9/NlM935IzpHYw+we+YR+Nz6xHJwwj test-PC"
  serviceAccountJSON: |
    {
        "id": "fdsfa",
        "service_account_id": "asdfdsaf",
        "created_at": "2020-01-14T12:04:10Z",
        "key_algorithm": "RSA_2048",
        "public_key": "-----BEGIN PUBLIC KEY-----\asdga/\nq4LJ+TBMDPRvZZkGBdTFCtfjR8lePTtpIZBjKEpPfKm8sQVldnr6BjGKVwRIeDKL\n44fpQI6g2/jwyPGnwytk9PTDD5YLCBRcoBpIANL9LVdBEFC4IkG5WZEyGBmX7iaJ\nb7osPcnB/SWfZ1uRyDHqbMzwQk/CKzTpcTfIYbYxzxSFOmCF/xugvw3q7WQR809e\n+fds+PvrzeWe5y3EIq7Xdtc/d\nCUR7YRICdYC3mSP18i+Ba2rr88sJ7u6pM3y95C48AIz2eB0qT1g3VEcXMH8skZkY\nPQIDAQAB\n-----END PUBLIC KEY-----\n",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcfsdwkQOomuvZLTRewhUI33QknVcRH0+5zrri+KsM1GYm8oUHKF4Mj\nRIwb9+cNZI62uQWU42YsEjj7anSA0zyEkwoywUtaJLuZZuPUofu/JhNjmV24X0QL\nwFb9P6XAyQh6Qpx6JFwoai+ZaCC46eVM1wpjXorlAoGAKg9efq+s4GTEYHgjqmGO\nJfLrlD1uxOc/bLu0e/ltzHPVTPUnTGNGxEYyoq8EZC/EPEAUmhBvBU90Uufgg7lX\nhLP38jY/2pn/YdYsqDJMoknN4FZ2EKjzhE6D63coaa/JAb3MBvaOuQAcAhIgsZmF\nvOBuvTgdO06nz+DSAwrxlOE=\n-----END PRIVATE KEY-----\n"
    }
```

### YandexInstanceClass custom resource

Ресурс описывает параметры группы Yandex Instances, которые будет использовать machine-controller-manager из модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `platformID` — тип платформы instances. [Список существующих платформ](https://cloud.yandex.com/docs/compute/concepts/vm-platforms).
  * Формат — строка.
* `cores` — количество ядер у создаваемых инстансов.
  * Формат — integer.
* `coreFraction` - базовый уровень производительности каждого ядра CPU у создаваемых инстансов. [Подробнее об уровнях производительности](https://cloud.yandex.ru/docs/compute/concepts/performance-levels).
  * Формат — integer.
  * По-умолчанию `100`.
  * Допустимые значения `0`, `5`, `20`, `50`, `100`.
* `memory` — количество оперативной памяти в мебибайтах у создаваемых инстансов.
  * Формат — integer.
* `gpus` — количество графических адаптеров у создаваемых инстансов.
  * Формат — integer.
* `imageID` — идентификатор образа, который будет установлен в заказанные instance'ы.
  * Формат — строка.
  * Найти нужный образ можно с помощью команды: `yc compute image list --folder-id standard-images | grep ubuntu-1804-lts`
* `preemptible` — Заказывать ли preemptible instance.
  * Формат — bool.
  * По-умолчанию `false`.
  * Опциональный параметр.
* `diskType` — тип диска у инстансов. [Типы дисков](https://cloud.yandex.com/docs/compute/concepts/disk#disks_types).
  * Формат — строка.
  * По-умолчанию `network-ssd`.
  * Опциональный параметр.
* `diskSizeGB` — размер диска у инстансов.
  * Формат — integer. В ГиБ.
  * По-умолчанию `50` ГиБ.
  * Опциональный параметр.
* `assignPublicIPAddress` - Присваивать ли публичные ip адреса инстансам потерять.
  * Формат — bool.
  * По-умолчанию `false`.
  * Опциональный параметр.
* `mainSubnet` — позволяет переопределить имя основного subnet, к которому будет подключен инстанс, по-умолчанию
используется subnet для зоны из конфига deckhouse `zoneToSubnetIdMap`.
  * Формат — string.
  * Пример — `e9bnc7g9mu9mper9clk4`
* `additionalSubnets` — список subnet, которые будут подключены к инстансу.
  * Формат — массив строк.
  * Пример:

    ```yaml
    - enp6t4snovl2ko4p15em
    - enp34dkcinm1nr5999lu
    ```

* `labels` — Метки инстанса
  * Формат — key:value
  * Опциональный параметр.

#### Пример YandexInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: YandexInstanceClass
metadata:
  name: test
spec:
  platformID: standard-v2
  cores: 4
  memory: 8192
  imageID: fd8rc75pn12fe3u2dnmb
```

### Storage

Storage настраивать не нужно, модуль автоматически создаст 2 StorageClass'а, покрывающие все варианты дисков в Yandex: hdd или ssd.

1. `network-hdd`
2. `network-ssd`

#### Важная информация об увеличении размера PVC

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer, CSI и Yandex.Cloud API, после увеличения размера PVC нужно:

1. Выполнить `kubectl cordon нода_где_находится_pod`;
2. Удалить Pod;
3. Убедиться, что ресайз произошёл успешно. В объекте PVC *не будет* condition `Resizing`. **Внимание!** `FileSystemResizePending` не является проблемой;
4. Выполнить `kubectl uncordon нода_где_находится_pod`.

### LoadBalancer

Модуль подписывается на Service объекты с типом LoadBalancer и создаёт соответствующие NetworkLoadBalancer и TargetGroup в Yandex.Cloud.

Больше информации в [документации](https://github.com/flant/yandex-cloud-controller-manager) CCM.

## Настройка окружения
### Автоматизированная подготовка окружения

1. [Terraform](https://github.com/deckhouse/deckhouse/tree/master/install-kubernetes/yandex/tf) для создания облачных ресурсов.
2. [Ansible playbook](https://github.com/deckhouse/deckhouse/tree/master/install-kubernetes/yandex/ansible) для provision'а master'а с помощью kubeadm.

## Как мне поднять кластер

1. Настройте облачное окружение. Возможно, [автоматически](#автоматизированная-подготовка-окружения).
2. [Установите](#включение-модуля) deckhouse с помощью `install.sh`, передав флаг `--extra-config-map-data base64_encoding_of_custom_config` с [параметрами](#параметры) модуля.
3. [Создайте](#yandexinstanceclass-custom-resource) один или несколько `YandexInstanceClass`
4. Управляйте количеством и процессом заказа машин в облаке с помощью модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/).

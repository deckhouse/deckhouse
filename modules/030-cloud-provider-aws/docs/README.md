---
title: "Модуль cloud-provider-aws"
---

## Содержимое модуля

1. cloud-controller-manager — контроллер для управления ресурсами облака из Kubernetes.
    * Создаёт route'ы для PodNetwork в cloud provider'е.
    * Создаёт LoadBalancer'ы для Service-объектов Kubernetes с типом LoadBalancer.
    * Синхронизирует метаданные AWS Instances и Kubernetes Nodes. Удаляет из Kubernetes ноды, которых более нет в AWS.
2. simple-bridge — DaemonSet. Настраивает bridge между нодами.
3. CSI storage — для заказа дисков в AWS.
4. Регистрация в модуле [node-manager]({{ site.baseurl }}/modules/040-node-manager/), чтобы [AWSInstanceClass'ы](#awsinstanceclass-custom-resource) можно было использовать в [NodeGroups]({{ site.baseurl }}/modules/040-node-manager/#nodegroup-custom-resource).

## Конфигурация

### Включение модуля

Модуль по-умолчанию **выключен**. Работает с версии Kubernetes 1.16.x. Для включения:

1. Корректно [настроить](#настройка-окружения) окружение.
2. Инициализировать deckhouse, передав параметр install.sh — `--extra-config-map-data base64_encoding_of_custom_config`.
3. Настроить параметры модуля.

### Параметры

> **Внимание!** При изменении конфигурационных параметров приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `NodeGroup` и `AWSInstanceClass`. См. подробнее в документации модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/#как-мне-перекатить-машины-с-новой-конфигурацией).

Для работы machine-controller-manager необходим пользователь с правами:
```
AmazonEC2FullAccess
AmazonVPCFullAccess
IAMFullAccess
```

* `providerAccessKeyId` — access key [ID](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html#access-keys-and-secret-access-keys)
* `providerSecretAccessKey` — access key [secret](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html#access-keys-and-secret-access-keys)
* `region` — имя AWS региона, в котором будут заказываться instances.
* `zones` — Список зон из `region`, где будут заказываться instances. Является значением по-умолчанию для поля zones в [NodeGroup]({{ site.baseurl }}/modules/040-node-manager/#nodegroup-custom-resource) объекте.
    * Формат — массив строк.
    * Опциональный параметр. Вычисляется из всех зон в текущем регионе.
* `instances` — параметры заказываемых instances.
    * `iamProfileName` — имя [Instance Profile](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html). В текущий момент модули deckhouse не требуют никаких дополнительных прав, поэтому здесь можно использовать "бесправную" роль.
        * Формат — строка.
    * `securityGroupIDs` — список дополнительных security groups, которые будут установлены на заказанные instances.
        * Формат — массив строк.
        * Опциональный параметр.
    * `extraTags` — список дополнительных тэгов, приклепляемых к каждому созданному Instance.
        * Формат — массив строк. **Внимание!** Обязательно должен содержать shared tag из [настройки](#настройка-окружения) облачного окружения.
* `keyName` — имя [SSH ключа](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html), предварительно загруженного в AWS, которое будет использовано для пользователя по-умолчанию.
    * Формат — строка.

#### Пример конфигурации:

```yaml
cloudProviderAwsEnabled: "true"
cloudProviderAws: |
  providerAccessKeyId: AKIAREDACTED
  providerSecretAccessKey: 5nJ45re/4daP/cted5hPxAI0mOehsd23sdC3
  region: eu-central-1
  instances:
    iamProfileName: kube-node
    securityGroupIDs:
    - sg-0e528731e3f4484a9
    extraTags:
      kubernetes.io/cluster/kube: shared
  keyName: kube
```

### AWSInstanceClass custom resource

Ресурс описывает параметры группы AWS Instances, которые будет использовать machine-controller-manager из модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `instanceType` — тип заказываемых instances. **Внимание!** Следует убедиться, что указанный тип есть во всех зонах, указанных в `zones`.
* `ami` — образ, который поставится в заказанные instance'ы.
    * Формат — строка, AMI ID.
    * Как найти нужный AMI: `aws ec2 --region <REGION> describe-images --filters 'Name=name,Values=buntu/images/hvm-ssd/ubuntu-bionic-18.04-amd64-server-2020*' | jq '.Images[].ImageId'` (В каждом регионе AMI разные).
* `spotPrice` — создавать ли spot instance'ы. Если указана пустая строка, то автоматически формируются минимально возможная цена за час, чтобы spot instance смог запуститься.
    * Формат — строка.
    * По-умолчанию создаются On-Demand instances.
* `diskType` — тип созданного диска.
    * По-умолчанию `gp2`.
    * Опциональный параметр.
* `iops` — количество iops. Применяется только для `diskType` **io1**.
    * Формат — integer.
    * Опциональный параметр.
* `diskSizeGb` — размер root диска.
    * Формат — integer. В ГиБ.
    * По-умолчанию `20` ГиБ.
    * Опциональный параметр.

#### Пример AWSInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: AWSInstanceClass
metadata:
  name: worker
spec:
  instanceType: t3.large
  ami: ami-040a1551f9c9d11ad
  diskSizeGb: 15
  diskType:  gp2
```

### Storage

По умолчанию создаётся один (default) StorageClass с именем `gp2` и типом диска `gp2`.

Создать StorageClass с типом диска `io1` можно, например, так:

```yaml
---
apiVersion: storage.k8s.io/v1beta1
kind: StorageClass
metadata:
  name: io1
provisioner: ebs.csi.aws.com
parameters:
  type: io1
  iopsPerGB: "20"
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer # обязательно!
```

Список всех возможных `parameters` для EBS CSI драйвера представлен в его [документации](https://github.com/kubernetes-sigs/aws-ebs-csi-driver).

### LoadBalancer

#### Аннотации объекта Service

Поддерживаются следующие параметры в дополнение к существующим в upstream:

1. `service.beta.kubernetes.io/aws-load-balancer-type` — может иметь значение `none`, что приведёт к созданию **только** Target Group, без какого либо LoadBalanacer.
2. `service.beta.kubernetes.io/aws-load-balancer-backend-protocol` — используется в связке с `service.beta.kubernetes.io/aws-load-balancer-type: none`.
   * Возможные значения:
     * `tcp`
     * `tls`
     * `http`
     * `https`
   * По-умолчанию, `tcp`.
   * **Внимание!** При изменении поля cloud-controller-manager попытается пересоздать Target Group. Если к ней уже привязаны NLB или ALB, удалить Target Group он не сможет и будет пытаться вечно. Необходимо вручную отсоединить от Target Group NLB или ALB.

## Настройка окружения

### Определение cluster ID тэга

**Важно!**
Следует выбрать уникальную строку, идентифицирующую кластер в конкретном AWS регионе. Например, `kube-prod`.
Тэг, идентифицирующий облачный API объект, принадлежащий к данному кластеру будет выглядеть так:

```yaml
kubernetes.io/cluster/kube-prod: shared
```

### Создание и подготовка ресурсов

В AWS нужно создать:

1. По subnetwork в каждой зоне с опцией `auto-assign public IPv4 address`. Прикрепить [clusterID тэг](#определение-cluster-id-тэга).
2. Routing table с роутом до IGW. Прикрепить [clusterID тэг](#определение-cluster-id-тэга).
3. Security group, разрешающий всю коммуникацию между instances. Прикрепить [clusterID тэг](#определение-cluster-id-тэга).
4. Заказанный и настроенный master instance со следующими параметрами:

    1. Сеть включена в subnetwork из шага №1.
    2. Прикрепить [clusterID тэг](#определение-cluster-id-тэга).

    [Пример](https://github.com/deckhouse/deckhouse/blob/master/install-kubernetes/aws/playbook.yml) настройки ОС для master'а через kubeadm.

### Автоматизированная подготовка окружения

1. [Terraform](https://github.com/deckhouse/deckhouse/tree/master/install-kubernetes/aws/tf) для создания облачных ресурсов.
2. [Ansible playbook](https://github.com/deckhouse/deckhouse/blob/master/install-kubernetes/aws/playbook.yml) для provision'а master'а с помощью kubeadm.

**Внимание!** Перед использованием готовых скриптов, следует установить два плагина для Terraform и Ansible.

* https://github.com/nbering/terraform-provider-ansible
* https://github.com/nbering/terraform-inventory

Ctrl+C, Ctrl+V для установки обоих:

```shell
mkdir -p ~/.terraform.d/plugins/
(
  cd ~/.terraform.d/plugins/
  curl -L https://github.com/nbering/terraform-provider-ansible/releases/download/v1.0.3/terraform-provider-ansible-${terraform_provider_ansible_ostype}_amd64.zip > terraform-provider-ansible.zip
  unzip terraform-provider-ansible.zip
  mv ${terraform_provider_ansible_ostype}_amd64/* .
  rm -rf ${terraform_provider_ansible_ostype}_amd64/ terraform-provider-ansible.zip
)

curl -L https://github.com/nbering/terraform-inventory/releases/download/v2.2.0/terraform.py > ~/.ansible-terraform-inventory
chmod +x ~/.ansible-terraform-inventory
```

## Как мне поднять кластер

1. [Настройте](#настройка-окружения) облачное окружение. Возможно, [автоматически](#автоматизированная-подготовка-окружения).
2. [Установите](#включение-модуля) deckhouse с помощью `install.sh`, передав флаг `--extra-config-map-data base64_encoding_of_custom_config` с [параметрами](#параметры) модуля.
3. [Создайте](#awsinstanceclass-custom-resource) один или несколько `AWSInstanceClass`
4. Управляйте количеством и процессом заказа машин в облаке с помощью модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/).

# Модуль cloud-provider-gcp

## Содержимое модуля

1. cloud-controller-manager — контроллер для управления ресурсами облака из Kubernetes.
    * Создаёт route'ы для PodNetwork в cloud provider'е.
    * Создаёт LoadBalancer'ы для Service-объектов Kubernetes с типом LoadBalancer.
    * Синхронизирует метаданные GCP Instances и Kubernetes Nodes. Удаляет из Kubernetes ноды, которых более нет в GCP.
2. simple-bridge CNI — DaemonSet, который работает на **каждой** ноде кластера. Вытаскивает из API Kubernetes CIDR, выданный этой конкретной ноде и кладёт в виде CNI конфигурации в `/etc/cni/net.d/simple-bridge.conf`.
3. CSI storage — для заказа дисков в GCP.
4. Регистрация в модуле [cloud-instance-manager](modules/040-cloud-instance-manager), чтобы [GCPInstanceClass'ы](#GCPInstanceClass) можно было использовать в [CloudInstanceClass'ах](modules/040-cloud-instance-manager/README.md#cloudinstancegroup-custom-resource).

## Конфигурация

### Включение модуля

Модуль по-умолчанию **выключен**. Для включения:

1. Корректно [настроить](#настройка-окружения) окружение.
2. Инициализировать deckhouse, передав параметр install.sh — `--extra-config-map-data base64_encoding_of_custom_config`.
3. Настроить параметры модуля.

### Параметры

* `networkName` — имя VPC network в GCP, где будут заказываться instances.
* `subnetworkName` — имя subnet в VPC netwok `networkName`, где будут заказываться instances.
* `region` — имя GCP региона, в котором будут заказываться instances.
* `zones` — Список зон из `region`, где будут заказываться instances. Является значением по-умолчанию для поля zones в [CloudInstanceGroup](modules/040-cloud-instance-manager/README.md#CloudInstanceGroup-custom-resource) объекте.
    * Формат — массив строк.
* `extraInstanceTags` — Список дополнительных GCP tags, которые будут установлены на заказанные instances. Позволяют прикрепить к создаваемым instances различные firewall правила в GCP.
    * Формат — массив строк.
    * Опциональный параметр.
* `sshKey` — публичный SSH ключ.
    * Формат — строка, как из `~/.ssh/id_rsa.pub`.
* `serviceAccountKey` — ключ к Service Account'у с правами Project Admin.
    * Формат — строка c JSON.
    * [Как получить](https://cloud.google.com/iam/docs/creating-managing-service-account-keys#creating_service_account_keys).
* `disableExternalIP` — прикреплять ли внешний IPv4-адрес к заказанным instances. Если выставлен `true`, то необходимо создать [Cloud NAT](https://cloud.google.com/nat/docs/overview) в GCP.
    * Формат — bool. Опциональный параметр.
    * По-умолчанию `true`.

#### Пример конфигурации:

```yaml
cloudProviderGcpEnabled: "true"
cloudProviderGcp: |
  networkName: default
  subnetworkName: kube
  region: europe-north1
  zones:
  - europe-north1-a
  - europe-north1-b
  - europe-north1-c
  extraInstanceTags:
  - kube
  disableExternalIP: false
  sshKey: "ssh-rsa testetestest"
  serviceAccountKey: |
    {
      "type": "service_account",
      "project_id": "test",
      "private_key_id": "easfsadfdsafdsafdsaf",
      "private_key": "-----BEGIN PRIVATE KEY-----\ntesttesttesttest\n-----END PRIVATE KEY-----\n",
      "client_email": "test@test-sandbox.iam.gserviceaccount.com",
      "client_id": "1421324321314131243214",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test%test-sandbox.iam.gserviceaccount.com"
    }
```

### GCPInstanceClass custom resource

Ресурс описывает параметры группы GCP Instances, которые будет использовать machine-controller-manager из модуля [cloud-instance-manager](modules/040-cloud-instance-manager). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `machineType` — тип заказываемых instances. **Внимание!** Следует убедиться, что указанный тип есть во всех зонах, указанных в `zones`.
* `image` — образ, который поставится во заказанные instance'ы.
    * Формат — строка, полный путь до образа, пример: `projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911`.
    * **Внимание!** Сейчас поддерживается и тестируется только Ubuntu 18.04.
* `preemptible` — Заказывать ли preemptible instance.
    * Формат — bool.
    * По-умолчанию `false`.
    * Опциональный параметр.
* `diskType` — тип созданного диска.
    * По-умолчанию `pd-standard`.
    * Опциональный параметр.
* `diskSizeGb` — размер root диска.
    * Формат — integer. В ГиБ.
    * По-умолчанию `50` ГиБ.
    * Опциональный параметр.
* `cloudInitSteps` — параметры bootstrap фазы.
    * `version` — версия. По сути, имя директории [здесь](modules/040-cloud-instance-manager/cloud-init-steps).
        * По-умолчанию `ubuntu-18.04-1.0`.
        * **WIP!** Precooked версия требует специально подготовленного образа.
    * `options` — ассоциативный массив параметров. Уникальный для каждой `version` и описано в [`README.md`](modules/040-cloud-instance-manager/cloud-init-steps) соответствующих версий. Пример для [ubuntu-18.04-1.0](modules/040-cloud-instance-manager/cloud-init-steps/ubuntu-18.04-1.0):

        ```yaml
        options:
          kubernetesVersion: "1.15.3"
        ```

#### Пример GCPInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: GCPInstanceClass
metadata:
  name: test
spec:
  machineType: n1-standard-1
  image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911
```

### Storage

Storage настраивать не нужно, модуль автоматически создаст 4 StorageClass'а, покрывающие все варианты дисков в GCP: standard или ssd, region-replicated или not-region-replicated.

1. `pd-standard-not-replicated`
2. `pd-standard-replicated`
3. `pd-ssd-not-replicated`
4. `pd-ssd-replicated`

## Настройка окружения

В GCP нужно создать:

1. Выделенный subnetwork.
2. Firewall, разрешающий коммуникацию между instances.
3. Установить опцию `can_ip_forward` в `true` при создании master instance.
4. Заказанный и настроенный master instance со следующими параметрами:

    1. `security_tags`, как в firewall'е `destination_tags`.
    2. Сеть включена в subnetwork из шага №1.
    3. Service Account с project admin доступом.

5. [Пример](install-kubernetes/gcp/ansible/master.yaml) настройки ОС для master'а через kubeadm.

### Автоматизированная подготовка окружения

1. [Terraform](install-kubernetes/gcp/tf) для создания облачных ресурсов.
2. [Ansible playbook](install-kubernetes/gcp/ansible) для provision'а master'а с помощью kubeadm.

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
3. [Создайте](#GCPInstanceClass-custom-resource) один или несколько `GCPInstanceClass`
4. Управляйте количеством и процессом заказа машин в облаке с помощью модуля [cloud-instance-manager](modules/040-cloud-instance-manager).

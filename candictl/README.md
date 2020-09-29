---
title: Deckhouse CandI (Cluster and Infrastructure) 
permalink: /candi/candictl.html
---
```
========================================================================================
 _____             _     _                                ______                _ _____
(____ \           | |   | |                              / _____)              | (_____)
 _   \ \ ____ ____| |  _| | _   ___  _   _  ___  ____   | /      ____ ____   _ | |  _
| |   | / _  ) ___) | / ) || \ / _ \| | | |/___)/ _  )  | |     / _  |  _ \ / || | | |
| |__/ ( (/ ( (___| |< (| | | | |_| | |_| |___ ( (/ /   | \____( ( | | | | ( (_| |_| |_
|_____/ \____)____)_| \_)_| |_|\___/ \____(___/ \____)   \______)_||_|_| |_|\____(_____)
========================================================================================
```
__(здесь будет логотип)__

Приложение для развертывания Kubernetes-кластеров и управления их инфраструктурой.

Основные функции:
* Подготовка инфраструктуры на базе облачного провайдера (AWS, YandexCloud, OpenStack)
* Установка Kubernetes и необходимых для его работы компонентов (в облаке или на bare metal)
* Установка **Deckhouse** - оператора, который управляет Kubernetes-кластером и устанавливает дополнительные модули
* Создание дополнительных master-узлов или статических узлов, внесение изменений в их конфигурацию и удаление
* Отслеживание изменений в инфраструктуре облачных провайдеров, внесение в нее изменений

## Создание кластера Kubernetes

### Подготовительный этап
Первый этап при создании кластера - подготовка серверов. 
Поддерживаются операционные системы `ubuntu-16.04`, `ubuntu-18.04`, `centos-7`.


* **Bare Metal** - необходимо предоставить SSH-доступ и возможность выполнять действия с правами администратора (sudo, есть возможность ввести пароль)
* **Cloud Provider** - необходимо убедится, что candi поддерживает работу с вашим провайдером и предоставить дополнительную секцию с настройками для него
    * Если провайдер не поддерживается, вы можете подготовить сервера самостоятельно и воспользоваться вариантом установки для Bare Metal

### Конфигурация
Конфигурация описывается в виде одного YAML-файла с несколькими секциями. Обязательно указать две секции:
* `ClusterConfiguration` - основные параметры Kubernetes-кластеров: сети для подов и сервисов, версия Kubernetes, домен кластера.
* `InitConfiguration` - параметры, необходимые для первоначальной настройки, которые могут быть изменены в будущем (например конфигурация Deckhouse)

Пример настроек:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
  prefix: main
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.16"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1alpha1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.example.com/deckhouse
  registryDockerCfg: | # base64 encoded section of docker.auths {"registry.example.com":{"username":"oauth2","password":"token"}}
    ewogICJyZWdpc3RyeS5leGFtcGxlLmNvbSI6IHsKICAgICJ1c2VybmFtZSI6ICJvYXV0aDIiLAogICAgInBhc3N3b3JkIjogInRva2VuIgogIH0KfQo=
  releaseChannel: Stable
  configOverrides:
    global:
      clusterName: main
      project: pivot
```
Этих параметров достаточно для установки Kubernetes-кластера на Bare Metal. 
Единственным условием будет указание дополнительного параметра `--ssh-host` при запуске команды для создания кластера.

Для облачного провайдера необходимо в том же YAML-файле указать еще одну секцию:
* **${PROVIDER_NAME}ClusterConfiguration** - специфические настройки для работы с провайдером: доступы для работы с API облака, сеть адресов для узлов, параметры и количество создаваемых виртуальных серверов и т.п. 

Пример для облака на базе OpenStack:
```yaml
...
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa publicsshkeyhere"
standard:
  internalNetworkCIDR: 192.168.199.0/24
  internalNetworkDNSServers:
  - 8.8.8.8
  - 4.2.2.2
  internalNetworkSecurity: true
  externalNetworkName: public
masterNodeGroup:
  replicas: 1
  instanceClass:
    flavorName: m1.large
    imageName: ubuntu-18-04-cloud-amd64
    kubernetesDataVolumeType: "__DEFAULT__"
    rootDiskSize: 20
provider:
  authURL: https://cloud.example.com/v3/
  domainName: Default
  tenantName: xxx
  username: xxx
  password: xxx
  region: HetznerFinland
```
### Bootstrap кластера
Для установки кластера необходимо воспользоваться подготовленным docker-образом.
Для примера воспользуемся образом из registry компании Flant.

1. Скачиваем свежий образ с необходимого канала обновления (в примере используется канал обновлений Alpha)
    ```bash
    docker pull registry.flant.com/sys/antiopa/install:alpha
    ```
    > Для того чтобы скачать образ с registry.flant.com необходимо создать и использовать токен вашего пользователя
    https://docs.gitlab.com/ce/user/profile/personal_access_tokens.html#creating-a-personal-access-token
2.  Запускаем контейнер в режиме интерактивного терминала и монтируем к нему:
   * `config.yaml` - YAML-файл, содержащий конфигурацию разворачиваемого кластера
   ```bash
   docker run -it \
     -v $(pwd)/config.yaml:/config.yaml \
     -v $HOME/.ssh/:/tmp/.ssh/ \
     registry.flant.com/sys/antiopa/install:alpha \
     bash
   ```
   > Для пользователей MacOS нет необходимости монтировать папку .ssh в /tmp, для удобного использования можно смонтировать её в директорию основного пользователя `/root`
3. Запускаем установку кластера:
   ```bash
   candictl bootstrap \
     --ssh-user=ubuntu \
     --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
     --config=/config.yaml 
   ```
### Динамическое создание ресурсов
Во время создания кластера будет установлен оператор Deckhouse, который готов к работе сразу после окончания процесса. 
Оператор расширяет API Kubernetes при помощи [механизма CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

Используя флаг `--resources` для команды `bootstrap` можно указать путь до файла с манифестами дополнительных ресурсов, которые необходимо установить после создания кластера.

Пример файла:
```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInstanceClass
metadata:
  name: worker
spec:
  flavorName: m1.large
  imageName: ubuntu-18-04-cloud-amd64
  mainNetwork: standard
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Cloud
  cloudInstances:
    minPerZone: 2
    maxPerZone: 4
    classReference:
      kind: OpenStackInstanceClass
      name: worker
---
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main
spec:
  inlet: HostPort
  ingressClass: nginx
  controllerVersion: '0.26'
  hostPort:
    httpPort: 80
    httpsPort: 443
```
В данном примере создается дополнительная динамически расширяемая группа узлов и ingress-controller.

**Главная особенность** заключается в том, что ресурс `kind: IngressNginxController` не будет существовать в 
кластере, пока не появится хотя бы один узел помимо первого master-узла, что приведет к ошибке при попытке его создания.

Deckhouse-candi в этом случае создаст ресурсы `OpenStackInstanceClass` и `NodeGroup`, дождется возможности создавать 
ресурсы с типом `IngressNginxController` и только после этого создаст их.

> Процесс создания ресурсов можно запустить отдельно, воспользовавшись командой `bootstrap-phase create-resources`. 

## Converge инфраструктуры
Существует острая необходимость реагировать на появление изменений в инфраструктуре кластера при использовании облачных провайдеров.

При внесении ручных изменений велика вероятность, что кластер перестанет соответствовать изначально развернутой конфигурации, 
что может привести к появлению сотен различных конфигураций и затруднить управление кластерами.

1. Для приведения объектов в облаке к начальному состоянию существует команда `converge`. 
    Во время выполнения команды candictl:
    * Подключается к кластеру Kubernetes
    * Собирает информацию о текущем состоянии (state) для узлов (node) и базовой инфраструктуры
    * Запускает terraform для базовой инфраструктуры, используя конфигурацию кластера и файлы Terraform state из secret'а в кластере Kubernetes
    * Сохраняет новое состояние в кластер Kubernetes
    * Обрабатывает master-узлы и статические узлы
        * Если master-узлов или статических узлов в облаке меньше, чем указано в состоянии кластера, candictl сначала создает недостающие узлы
        * Для каждого узла вызывается повторное создание при помощи terraform и состояния узла из кластера Kubernetes
        * Если master-узлов или статических узлов в облаке больше, чем нужно, узлы будут удалены 
    > При расхождении состояния и при удалении объектов из облака candictl запросит подтверждение.

    Пример запуска:
    ```bash
    candictl converge \
      --ssh-host 8.8.8.8 \
      --ssh-user=ubuntu \
      --ssh-agent-private-keys=/tmp/.ssh/id_rsa
    ```

2. Для проверки наличия изменений в облаке и в состоянии terraform существуют команды:
    * `candictl terraform converge-exporter` - запускает экспортер для Prometheus, который  периодически проверяет схождение состояния в облаке и состояния terraform'а в секретах.
        > Эта команда используется в модуле `040-terraform-manager`
    * `candictl terraform check` - запускает проверку один раз и выдает отчет в формате YAML или JSON.


## Удаление кластера Kubernetes
Для удаления кластера, развернутого в облаке, используется команда `destroy`.
Во время выполнения команды candictl:
* Подключается к кластеру Kubernetes
* Удаляет ресурсы, которые отвечают за создание объектов в облаке: service'ы с типом LoadBalancer,  PV, PVC и Machines (candictl дожидается, пока ресурсы удалятся из кластера).
* Собирает информацию о текущем состоянии (state) для узлов (node) и базовой инфраструктуры
* По очереди вызывает удаление всех компонентов

Пример запуска:
```bash
candictl destroy \
  --ssh-host 8.8.8.8 \
  --ssh-user=ubuntu \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa
```

## Что такое Deckhouse?

**Deckhouse** — оператор Kubernetes от Флант — управляет Kubernetes в соответствии с нашими лучшими практиками.

## Примечание об использовании документации

Помните, что документацию нужно смотреть по той версии, которая установлена в кластере! Разрыв между версиями значителен и может ввести вас в заблуждение.

## Устройство и фичи Deckhouse

Deckhouse является расширением [addon-operator](https://github.com/flant/addon-operator/) и используется пока только внутри компании Флант, включая кластеры наших клиентов. 
По сути *Deckhouse*, это наш внутренний набор [модулей](https://github.com/flant/addon-operator/blob/master/MODULES.md) и [хуков](https://github.com/flant/addon-operator/blob/master/HOOKS.md) `addon-operator`.

*Также* есть bash-скрипт [***REMOVED***](https://github.com/deckhouse/deckhouse/blob/master/***REMOVED***), для массовых операций на кластерах.

### Что делают хуки и какие они бывают?

В основном, хуки делают одно или несколько из следующих действий:
 * выполняют некоторый *"discovery"* и генерируют Helm Values (или глобальные или для своего модуля), которые затем используются в Helm Chart'ах
 * вносят изменения в конфиг deckhouse (например, генерируют пароль, если его нет)
 * удаляют объекты (например, удаляют конфликтующие объекты перед установкой Helm Chart'а модуля)
 * некоторые глобальные хуки вносят изменения в те объекты, которые не находятся под управлением helm

Глобальные хуки в *Deckhouse* лежат в директории `global-hooks/*`, хуки модулей — в `modules/*/hooks/*`. Существует возможность привязать запуск хука к одному или нескольким [событиям](https://github.com/flant/addon-operator/blob/master/HOOKS.md#overview).

Для того чтобы посмотреть, когда будет запускаться конкретный хук — хук можно вызвать с параметром `--config`, при этом хук должен вернуть JSON в "интуитивно-понятном" формате. Подробнее про формат настройки смотри [тут](https://github.com/flant/addon-operator/blob/master/HOOKS.md#bindings)

Кроме обычных хуков у модуля может быть специальный детектор включенности (располагается в `modules/*/enabled`) — он выполняется после отработки глобальных хуков, но до запуска всех модулей. Подробнее про жизненный цикл модуля смотри [тут](https://github.com/flant/addon-operator/blob/master/LIFECYCLE.md).

## Использование и конфигурирование Deckhouse

### Как конфигурируется Deckhouse?

Конфиг *Deckhouse* расположен в ConfigMap с названием deckhouse в namespace d8-system:
```
kubectl -n d8-system edit cm/deckhouse
```

Этот ConfigMap имеет **особенную структуру** (ввиду ограничение ConfigMap в Kubernetes), обратите внимание на `|` и не запутайтесь:

```yaml
apiVersion: v1
metadata:
  name: deckhouse
  namespace: d8-system
data:
  global: |          # <--- очень важно, вертикальная черта!!!
    # Тут кусок Yaml-файла с глобальными настройками
    project: someproject
    clusterName: main
    modules:
      publicDomainTemplate: "%s.kube.domain.my"
  nginxIngress: |
    # Тут кусок Yaml-файла, касающийся модуля nginx-ingress
    config:
      hsts: true
  someModuleName: |  # <--- тут всегда camel case от названия модуля
    foo: bar
  dashboardEnabled: "false"   # <--- вот так можно отключить модуль
```

### Что можно и что нужно настраивать?

* глобальный конфиг описан в этом документе ниже,
* конфиг каждого модуля описан в документации модуля
* всегда четко указано, какие параметры *нужно обязательно* настроить

### Выделение узлов под определенный вид нагрузки

Для всех модулей принята единая стратегия:
1. Если параметр модуля `nodeSelector` не указан, то мы смотрим, есть ли в кластере узлы с определенными лейблами и если они есть – автоматически используем соответствующие nodeSelector'ы. Конкретные лейблы и порядок поиска узлов см. ниже.
1. Если параметр модуля `tolerations` не указан, то мы автоматически ставим pod'ам модуля все возможные toleration'ы (см. список ниже).
1. Отключить автоматическое вычисление параметров `nodeSelector` или `tolerations` можно значением `false`.

**Важно!** Если модуль предполагает работу DaemonSet'a на всех нодах кластера (например, `ping-exporter` и `node-problem-detector`) или модуль должен работать на master-узлах (например `prometheus-metrics-adapter` или некоторые компоненты `vertical-pod-autoscaler`) — то у таких модулей возможность настройки `nodeSelector` и `tolerations` отключена.

{% offtopic title="Особенности автоматики, зависящие от 'типа' модуля" %}{% raw %}
* Модули *monitoring* (operator-prometheus, prometheus и vertical-pod-autoscaler):
  * Порядок поиска узлов (для определения nodeSelector):
    * Наличие ноды с лейблом <code>node-role.deckhouse.io/MODULE_NAME</code>
    * Наличие ноды с лейблом <code>node-role.deckhouse.io/monitoring</code>
    * Наличие ноды с лейблом <code>node-role.deckhouse.io/system</code>
  * Добавляемые toleration'ы (добавляются одновременно все):
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code> 

      (Например: <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}</code>)
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}</code>
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>
* Модули *frontend* (исключительно nginx-ingress)
    * Порядок поиска узлов (для определения nodeSelector):
        * Наличие ноды с лейблом <code>node-role.deckhouse.io/MODULE_NAME</code>
        * Наличие ноды с лейблом <code>node-role.deckhouse.io/frontend</code>
    * Добавляемые toleration'ы (добавляются одновременно все):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"frontend"}</code>
* Все остальные модули
    * Порядок поиска узлов (для определения nodeSelector):
        * Наличие ноды с лейблом <code>node-role.deckhouse.io/MODULE_NAME</code> (Например: <code>node-role.deckhouse.io/cert-manager</code>)
        * Наличие ноды с лейблом <code>node-role.deckhouse.io/system</code>
    * Добавляемые toleration'ы (добавляются одновременно все):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code> (Например: <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"network-gateway"}</code>)
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>
{% endraw %}
{% endofftopic %}

## Конфигурация (глобальная)

### Что нужно настроить?

Нужно обязательно настроить **project** и **clusterName** и желательно настроить **modules.publicDomainTemplate**.

```yaml
global: |
  project: projectname
  clusterName: main
  modules:
    publicDomainTemplate: "%s.kube.company.my"
```

### Параметры

* `project` (обязательно) — имя проекта.
* `clusterName` (обязательно) — имя кластера (должно соответствовать имени кластера в `***REMOVED***_registry`).
* `modules` — параметры для служебных компонентов;
  * `publicDomainTemplate` (желательно) — шаблон c ключом "%s" в качестве динамической части строки. Будет использоваться для образования служебных доменов (например, `%s.kube.domain.my`). Если параметр не указан, то Ingress-ресурсы создаваться не будут.
  * `ingressClass` — класс Ingress-контроллера, который используется для служебных компонентов.
    * По умолчанию `nginx`.
  * `placement` — настройки, определяющие расположение компонентов Deckhouse.
    * `customTolerationKeys` — список ключей пользовательских taint'ов, необходимо указывать, чтобы позволить выезжать на выделенные ноды критическим add-on'ам, таким как например cni и csi.
      * Пример:

        ```yaml
        customTolerationKeys:
        - dedicated.example.com
        - node-dedicated.example.com/master
        ```
  * `https` — способ реализации HTTPS, используемый служебными компонентами.
    * `mode` — режим работы HTTPS:
      * `Disabled` — в данном режиме все служебные компоненты будут работать только по http (некоторые модули могут не работать, например [user-authn](/modules/150-user-authn));
      * `CertManager` — все служебные компоненты будут работать по https и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
      * `CustomCertificate` — все служебные компоненты будут работать по https используя сертификат из namespace `d8-system`;
      * `OnlyInURI` — все служебные компоненты будут работать по http (подразумевая, что перед ними стоит внешний https-балансер, который терминирует https).
      * По умолчанию `CertManager`.
    * `certManager`
      * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для служебных компонентов (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По умолчанию `letsencrypt`.
    * `customCertificate`
      * `secretName` - указываем имя secret'а в namespace `d8-system`, который будет использоваться для системных компонентов (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets).
        * По умолчанию `false`.
  * `resourcesRequests` - количество CPU и memory, выделяемые для работы служебных компонентов.
    * `everyNode` - системные компоненты, работающие на каждом узле кластера (обычно это DaemonSet'ы).
      * `cpu` – сумма CPU, выделяемая для всех компонентов на каждом узле (по умолчанию: 300m)
      * `memory` – суммарный объем памяти, выделяемый для всех компонентов на каждом узле (по умолчанию: 512Mi)
    * `masterNode` - системные компоненты (control plane и системные компоненты на мастер-узлах).
      * `cpu` – сумма CPU, выделяемая для системных компонентов на мастер-узлах сверх `everyNode`.
        * Для кластера, управляемого Deckhouse, значение по умолчанию определяется автоматически: `.status.allocatable.cpu` минимального мастер-узла (но не более 4 ядер) минус `everyNode`.
        * Для managed-кластера значение по умолчанию - 1 ядро минус `everyNode`.
      * `memory` – суммарный объем памяти, выделяемый для системных компонентов на мастер-узлах сверх `everyNode`.
        * Для кластера, управляемого Deckhouse, значение по умолчанию определяется автоматически: `.status.allocatable.memory` минимального мастер-узла (но не более 8 Гб) минус `everyNode`.
        * Для managed-кластера значение по умолчанию - 1 Гб минус `everyNode`.
      * **Внимание!** В случае managed-кластера Deckhouse не управляет control-plane компонентами, поэтому все ресурсы отдаются системным компонентам.  
 * `storageClass` — имя storage class, который будет использоваться для всех служебных компонентов (prometheus, grafana, openvpn, ...).
    * По умолчанию — null, а значит служебные будут использовать `cluster.defaultStorageClass` (который определяется автоматически), а если такого нет — `emptyDir`.
    * Этот параметр имеет смысл использовать только в исключительных ситуациях.
* `highAvailability` — глобальный включатель режима отказоустойчивости для модулей, которые это поддерживают. По умолчанию не определён и решение принимается на основе autodiscovery-параметра `global.discovery.clusterControlPlaneIsHighlyAvailable`.

     
### Включение и отключение модулей

Deckhouse устанавливает только включённые [модули](https://github.com/flant/addon-operator/blob/master/MODULES.md). Смотри подробнее про алгоритм определения включенности модуля [тут](https://github.com/flant/addon-operator/blob/master/LIFECYCLE.md#modules-discovery).

# Разворачивание и управление узлами в кластерах Kubernetes
Для создания новых кластеров и управления узлами используется подсистема [candi (Cluster and Infrastructure)](/candi/). Ключевые преимущества:
* Единый процесс установки и управления кластерами Kubernetes для baremetal и cloud инсталляций
* Простое управляемое обновление:
   * компонентов kubernetes (control-plane, kubelet)
   * компонентов системы (ядро, докер, прочие пакеты)
* Декларативный стиль описания всех компонентов инфраструктуры кластера в процессе установки и использования

**Candi включает в себя**:
* Ядро общего функционала, используемое в модулях и инсталляторе:
    * Набор простых идемпотентных скриптов на bash, которые конфигурируют узлы (см. подробнее [bashible](/candi/bashible/)).
    * Шаблоны конфигурации kubeadm (kubeadm-config.yaml и kustomize патчи), которыми конфигурируется control-plane.
    * Для каждого поддерживаемого облачного провайдера – необходимые тераформы и дополнительные скрипты конфигурации узлов.

* Инсталлятор кластера и deckhouse:
    * В облаках installer использует terraform для создания инфраструктуры и отдельный terraform для создания первого узла (при установке происходит два запуска!)
    * State terraform'а, оставшийся после создания базовой инфраструктуры, сохраняется в кластер в namespace `kube-system` в secret `d8-terraform-state`.

* Модуль [control-plane-manager](/modules/040-control-plane-manager) — реализация `managed` control plane:
    * При использовании этого модуля обновление и настройка компонентов control plane полностью переходят под управление Deckhouse. 
    * Обновление patch-версии будет происходить автоматически при релизах Deckhouse. Точность версии в конфигурации можно указать только до minor-версии (например `1.16`).
        * В Deckhouse для каждой поддерживаемой минорной версии определена **точная версия**. Версия кластера `1.15`, точная версия `1.15.9`.
        * Точная версия может не совпадать с максимально доступной версией в репозитории kubernetes.
    * Работает как для singlemaster, так и для multimaster кластеров. Позволяет добавлять в кластер новые master-узлы и удалять старые.

* Модуль [node-manager](/modules/040-node-manager) — реализации `managed` узлов (нод).
    * Работает как в облаке, так и в baremetal кластерах.
    * Поддерживаемые типы узлов: Static, Hybrid, Cloud - подробнее о каждом типе написано в документации модуля.
    * Умно и безопасно по одному обновляет (или перекатывает) узлы при изменении настроек (например версии докера, ядра).
    * Позволяет пользователю отключить автоматическое обновление узлов и самостоятельно контролировать процесс, оповещает о необходимом обновления при помощи алертов.
    * Для управления узлами в кластере используется специальный ресурс - `NodeGroup`.
    * Настройка узла и управление им реализованы при помощи [bashible](/candi/bashible/).

* Модули Deckhouse `cloud-provider-` для взаимодействия с облачной инфраструктурой:
    * Провайдеры, которые полностью поддерживают candi:

        | Провайдер     | Варианты установки |
        | ------------- | ------------------ |
        | [cloud-provider-openstack](/modules/030-cloud-provider-openstack/)  | [layouts](/candi/cloud-providers/openstack/) |
    
    * Необходимую информацию для подключения к API и настройки cloud-provider'ы берут из secret'ов в namespace `kube-system`, либо из настроек модуля.


### Разворачивание кластера и установка Deckhouse:

Разворачивание кластера производится при помощи [специального приложения](/candi/candictl.html) `candictl` (или installer).
Installer принимает на вход единственный YAML-файл, в котором описана конфигурация для развертывания кластера.

{% raw %}
Сокращенный пример файла конфигурации:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.16"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1alpha1
kind: InitConfiguration
sshPublicKeys:
- ...
masterNodeGroup:
  ...
deckhouse:
  imagesRepo: registry.flant.com/sys/antiopa
  registryDockerCfg: ...
  releaseChannel: Alpha
  bundle: Default
  configOverrides:
    global:
      clusterName: main
      project: pivot
```
{% endraw %}
Так же необходимо указать параметры подключения по ssh, чтобы попасть на сервер для подготовки инфраструктуры и установки deckhouse.

Пример команды запуска установки кластера:
```bash
candictl bootstrap \
  --ssh-user=ubuntu \
  --ssh-agent-private-keys=~/.ssh/tfadm-id-rsa \
  --ssh-bastion-user=y.gagarin \
  --ssh-bastion-host=tf.hf-bastion \
  --config=/config.yaml 
```
Для удобного запуска подготовлен специальный Docker-образ.

Обратите внимание на поле `deckhouse.bundle` в InitConfiguration. Выбранный bundle определяет устанавливаемые по умолчанию модули Deckhouse. Подробнее читайте в [документации модуля deckhouse](/modules/020-deckhouse/).

#### Варианты установки Deckhouse:
Сейчас Deckhouse поддерживает 4 варианта установки:
* **Установка в baremetal-кластера** - candictl подключается к подготовленному серверу по SSH, устанавливает зависимости, последнее ядро linux, docker и control-plane, после чего устанавливает Deckhouse. 
   * В конфигурации необходимо указать `InitConfiguration` и `ClusterConfiguration` с `clusterType: Static`
   * Выбрать bundle - `Default`

* **Установка в облако** - candictl при помощи Terraform в облаке создает виртуальную машину, после чего подключается к ней по SSH и выполняет те же действия, что и для baremetal-кластера.
   * В конфигурации необходимо указать `InitConfiguration` и `ClusterConfiguration` с `clusterType: Cloud`
   * Так же конфигурации необходимо указать секции, специфичные для вашего облачного провайдера (дл OpenStack это будут `OpenStackInitConfiguration` и `OpenStackClusterConfiguration`)
   * Выбрать bundle - `Default`

* _Coming_Soon_: **Установка в managed-кластера (EKS, GKE и другие)** - TODO
   * В конфигурации необходимо указать - TODO
   * Выбрать bundle - `Managed`

* _Coming_Soon_: **Установка в уже существующий кластер** - candictl подключается к уже работающему Kubernetes-кластеру и устанавливает Deckhouse. 
   * В конфигурации необходимо указать - TODO
   * Выбрать bundle - `Minimal`

## Ведение разработки

[Читай документ для разработчиков](/guides/development.html)

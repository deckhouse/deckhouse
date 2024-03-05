---
title: "Cloud provider — AWS"
---

Взаимодействие с облачными ресурсами провайдера [AWS](https://aws.amazon.com/) осуществляется с помощью модуля `cloud-provider-aws`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы AWS при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-aws`:
- Управляет ресурсами AWS с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне AWS.
  * Создает LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  * Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации и удаляет из кластера узлы, которых более нет в AWS.
- Заказывает диски в AWS с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [AWSInstanceClass'ы](cr.html#awsinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).

---
title: "Cloud provider — Azure"
---

Взаимодействие с облачными ресурсами провайдера [Azure](https://portal.azure.com/) осуществляется с помощью модуля `cloud-provider-azure`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы Azure при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-azure`:
- Управляет ресурсами Azure с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне Azure.
  * Создает LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  * Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации и удаляет из кластера узлы, которых уже нет в Azure.
- Заказывает диски в Azure с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
- Регистрируется в модуле [`node-manager`](../../modules/040-node-manager/), чтобы [AzureInstanceClass'ы](cr.html#azureinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).

> **Внимание!** При использовании балансировщиков нагрузки исходящий трафик также идет через них. Если ни у одного балансировщика нет правил для UDP, весь исходящий UDP-трафик блокируется, вследствие чего не работают такие утилиты, как `ntpdate` и `chrony`. Для решения проблемы необходимо самостоятельно добавить load balancing rule с любым UDP-портом к уже существующему балансировщику либо в кластере создать сервис с типом LoadBalancer с любым UDP-портом.

---
title: "Cloud provider — GCP"
---

Взаимодействие с облачными ресурсами провайдера [Google](https://cloud.google.com/) осуществляется с помощью модуля `cloud-provider-gcp`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы GCP при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-gcp`:
- Управляет ресурсами GCP с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне GCP.
  * Создает LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  * Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации и удаляет из кластера узлы, которых уже нет в GCP.
- Заказывает диски в GCP с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [GCPInstanceClass'ы](cr.html#gcpinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).

---
title: "Cloud provider — Yandex Cloud"
---

Взаимодействие с облачными ресурсами провайдера [Yandex Cloud](https://cloud.yandex.ru/) осуществляется с помощью модуля `cloud-provider-yandex`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы Yandex Cloud при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-yandex`:
- Управляет ресурсами Yandex Cloud с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне Yandex Cloud.
  * Актуализирует метаданные Yandex Cloud Instances и Kubernetes Nodes. Удаляет из Kubernetes узлы, которых уже нет в Yandex Cloud.
- Заказывает диски в Yandex Cloud с помощью компонента `CSI storage`.
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [YandexInstanceClass'ы](cr.html#yandexinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).

## Интеграция с Yandex Cloud

### Настройка групп безопасности

При создании [облачной сети](https://cloud.yandex.ru/ru/docs/vpc/concepts/network#network), Yandex Cloud создаёт [группу безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups) по умолчанию для всех подключенных сетей, включая сеть кластера Deckhouse Kubernetes Platform. Эта группа безопасности по умолчанию содержит правила разрешающие любой входящий и исходящий трафик и применяется для всех подсетей облачной сети, если на объект (интерфейс ВМ) явно не назначена другая группа безопасности.

{% alert level="danger" %}
Не удаляйте правила по умолчанию, разрешающие любой трафик, до того как закончите настройку правил группы безопасности. Это может нарушить работоспособность кластера.
{% endalert %}

Здесь приведены общие рекомендации по настройке группы безопасности. Некорректная настройка групп безопасности может сказаться на работоспособности кластера. Пожалуйста ознакомьтесь с [особенностями работы групп безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups#security-groups-notes) в Yandex Cloud перед использованием в продуктивных средах.

1. Определите облачную сеть, в которой работает кластер Deckhouse Kubernetes Platform.

   Название сети совпадает с полем `prefix` ресурса [ClusterConfiguration](../../installing/configuration.html#clusterconfiguration). Его можно узнать с помощью команды:

   ```bash
   kubectl get secrets -n kube-system d8-cluster-configuration -ojson | \
     jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix | cut -d: -f2
   ```

1. В консоли Yandex Cloud выберите сервис Virtual Private Cloud и перейдите в раздел *Группы безопасности*. У вас должна отображаться одна группа безопасности с пометкой `Default`.

    ![Группа безопасности по умолчанию](../../images/030-cloud-provider-yandex/sg-ru-default.png)

1. Создайте правила согласно [инструкции Yandex Cloud](https://cloud.yandex.ru/ru/docs/managed-kubernetes/operations/connect/security-groups#rules-internal).

    ![Правила для группы безопасности](../../images/030-cloud-provider-yandex/sg-ru-rules.png)

1. Удалите правило, разрешающее любой **входящий** трафик (на скриншоте выше оно уже удалено), и сохраните изменения.

### Интеграция с Yandex Lockbox

С помощью инструмента [External Secrets Operator](https://github.com/external-secrets/external-secrets) вы можете настроить синхронизацию секретов [Yandex Lockbox](https://cloud.yandex.com/ru/docs/lockbox/concepts/) с секретами кластера Deckhouse Kubernetes Platform.

Приведенную инструкцию следует рассматривать как *Быстрый старт*. Для использования интеграции в продуктивных средах ознакомьтесь со следующими ресурсами:

- [Yandex Lockbox](https://cloud.yandex.ru/ru/docs/lockbox/)
- [Синхронизация с секретами Yandex Lockbox](https://cloud.yandex.ru/ru/docs/managed-kubernetes/tutorials/kubernetes-lockbox-secrets)
- [External Secret Operator](https://external-secrets.io/latest/)

#### Инструкция по развертыванию

1. [Создайте сервисный аккаунт](https://cloud.yandex.com/ru/docs/iam/operations/sa/create), необходимый для работы External Secrets Operator:

   ```shell
   yc iam service-account create --name eso-service-account
   ```

1. [Создайте авторизованный ключ](https://cloud.yandex.ru/ru/docs/iam/operations/authorized-key/create) для сервисного аккаунта и сохраните его в файл:

   ```shell
   yc iam key create --service-account-name eso-service-account --output authorized-key.json
   ```

1. [Назначьте](https://cloud.yandex.ru/ru/docs/iam/operations/sa/assign-role-for-sa) сервисному аккаунту [роли](https://cloud.yandex.com/ru/docs/lockbox/security/#service-roles) `lockbox.editor`, `lockbox.payloadViewer` и `kms.keys.encrypterDecrypter` для доступа ко всем секретам каталога:

   ```shell
   folder_id=<идентификатор каталога>
   yc resource-manager folder add-access-binding --id=${folder_id} --service-account-name eso-service-account --role lockbox.editor
   yc resource-manager folder add-access-binding --id=${folder_id} --service-account-name eso-service-account --role lockbox.payloadViewer
   yc resource-manager folder add-access-binding --id=${folder_id} --service-account-name eso-service-account --role kms.keys.encrypterDecrypter
   ```

   Для более тонкой настройки ознакомьтесь с [управлением доступом в Yandex Lockbox](https://cloud.yandex.com/ru/docs/lockbox/security).

1. Установите External Secrets Operator с помощью Helm-чарта согласно [инструкции](https://cloud.yandex.com/ru/docs/managed-kubernetes/operations/applications/external-secrets-operator#helm-install).

   Обратите внимание, что вам может понадобиться задать `nodeSelector`, `tolerations` и другие параметры. Для этого используйте файл `./external-secrets/values.yaml` после распаковки Helm-чарта.

   Скачайте и распакуйте чарт:

   ```shell
   helm pull oci://cr.yandex/yc-marketplace/yandex-cloud/external-secrets/chart/external-secrets \
     --version 0.5.5 \
     --untar
   ```

   Установите Helm-чарт:

   ```shell
   helm install -n external-secrets --create-namespace \
     --set-file auth.json=authorized-key.json \
     external-secrets ./external-secrets/
   ```

   Где:
   - `authorized-key.json` — название файла с авторизованным ключом из шага 2.

1. Создайте хранилище секретов [SecretStore](https://external-secrets.io/latest/api/secretstore/), содержащее секрет `sa-creds`:

   ```shell
   kubectl -n external-secrets apply -f - <<< '
   apiVersion: external-secrets.io/v1alpha1
   kind: SecretStore
   metadata:
     name: secret-store
   spec:
     provider:
       yandexlockbox:
         auth:
           authorizedKeySecretRef:
             name: sa-creds
             key: key'
   ```

   Где:
   - `sa-creds` — название `Secret`, содержащий авторизованный ключ. Этот секрет должен появиться после установки Helm-чарта.
   - `key` — название ключа в поле `.data` секрета выше.

#### Проверка работоспособности

1. Проверьте статус External Secrets Operator и созданного хранилища секретов:

   ```shell
   $ kubectl -n external-secrets get po
   NAME                                                READY   STATUS    RESTARTS   AGE
   external-secrets-55f78c44cf-dbf6q                   1/1     Running   0          77m
   external-secrets-cert-controller-78cbc7d9c8-rszhx   1/1     Running   0          77m
   external-secrets-webhook-6d7b66758-s7v9c            1/1     Running   0          77m

   $ kubectl -n external-secrets get secretstores.external-secrets.io 
   NAME           AGE   STATUS
   secret-store   69m   Valid
   ```

1. [Создайте секрет](https://cloud.yandex.ru/ru/docs/lockbox/operations/secret-create) Yandex Lockbox со следующими параметрами:

    - **Имя** — `lockbox-secret`.
    - **Ключ** — введите неконфиденциальный идентификатор `password`.
    - **Значение** — введите конфиденциальные данные для хранения `p@$$w0rd`.

1. Создайте объект [ExternalSecret](https://external-secrets.io/latest/api/externalsecret/), указывающий на секрет `lockbox-secret` в хранилище `secret-store`:

   ```shell
   kubectl -n external-secrets apply -f - <<< '
   apiVersion: external-secrets.io/v1alpha1
   kind: ExternalSecret
   metadata:
     name: external-secret
   spec:
     refreshInterval: 1h
     secretStoreRef:
       name: secret-store
       kind: SecretStore
     target:
       name: k8s-secret
     data:
     - secretKey: password
       remoteRef:
         key: <ИДЕНТИФИКАТОР_СЕКРЕТА>
         property: password'
   ```

   Где:

   - `spec.target.name` — имя нового секрета. External Secret Operator создаст этот секрет в кластере Deckhouse Kubernetes Platform и поместит в него параметры секрета Yandex Lockbox `lockbox-secret`.
   - `spec.data[].secretKey` — название ключа в поле `.data` секрета, который создаст External Secret Operator.
   - `spec.data[].remoteRef.key` — идентификатор созданного ранее секрета Yandex Lockbox `lockbox-secret`. Например, `e6q28nvfmhu539******`.
   - `spec.data[].remoteRef.property` — **ключ**, указанный ранее, для секрета Yandex Lockbox `lockbox-secret`.

1. Убедитесь, что новый ключ `k8s-secret` содержит значение секрета `lockbox-secret`:

   ```shell
   kubectl -n external-secrets get secret k8s-secret -ojson | jq -r '.data.password' | base64 -d
   ```

   В выводе команды будет содержаться **значение** ключа `password` секрета `lockbox-secret`, созданного ранее:

   ```shell
   p@$$w0rd
   ```

### Интеграция с Yandex Managed Service for Prometheus

С помощью данной интеграции вы можете использовать [Yandex Managed Service for Prometheus](https://cloud.yandex.ru/ru/docs/monitoring/operations/prometheus/) в качестве внешнего хранилища метрик, например, для долгосрочного хранения.

#### Запись метрик

1. [Создайте сервисный аккаунт](https://cloud.yandex.com/ru/docs/iam/operations/sa/create) с ролью `monitoring.editor`.
1. [Создайте API-ключ](https://cloud.yandex.ru/ru/docs/iam/operations/api-key/create) для сервисного аккаунта.
1. Создайте ресурс `PrometheusRemoteWrite`:

   ```shell
   kubectl apply -f - <<< '
   apiVersion: deckhouse.io/v1
   kind: PrometheusRemoteWrite
   metadata:
     name: yc-remote-write
   spec:
     url: <URL_ЗАПИСИ_МЕТРИК>
     bearerToken: <API_КЛЮЧ>
   '
   ```

   Где:

   - `<URL_ЗАПИСИ_МЕТРИК>` — URL со страницы Yandex Monitoring/Prometheus/Запись метрик.
   - `<API_КЛЮЧ>` — API-ключ, созданный на предыдущем шаге. Например, `AQVN1HHJReSrfo9jU3aopsXrJyfq_UHs********`.

   Также вы можете указать дополнительные параметры в соответствии с [документацией](../../modules/300-prometheus/cr.html#prometheusremotewrite).

Подробнее с данной функциональностью можно ознакомиться в [документации Yandex Cloud](https://cloud.yandex.ru/ru/docs/monitoring/operations/prometheus/ingestion/remote-write).

#### Чтение метрик через Grafana

1. [Создайте сервисный аккаунт](https://cloud.yandex.com/ru/docs/iam/operations/sa/create) с ролью `monitoring.viewer`.
1. [Создайте API-ключ](https://cloud.yandex.ru/ru/docs/iam/operations/api-key/create) для сервисного аккаунта.
1. Создайте ресурс `GrafanaAdditionalDatasource`:

   ```shell
   kubectl apply -f - <<< '
   apiVersion: deckhouse.io/v1
   kind: GrafanaAdditionalDatasource
   metadata:
     name: managed-prometheus
   spec:
     type: prometheus
     access: Proxy
     url: <URL_ЧТЕНИЕ_МЕТРИК_ЧЕРЕЗ_GRAFANA>
     basicAuth: false
     jsonData:
       timeInterval: 30s
       httpMethod: POST
       httpHeaderName1: Authorization
     secureJsonData:
       httpHeaderValue1: Bearer <API_КЛЮЧ>
   '
   ```

   Где:

   - `<URL_ЧТЕНИЕ_МЕТРИК_ЧЕРЕЗ_GRAFANA>` — URL со страницы Yandex Monitoring/Prometheus/Чтение метрик через Grafana.
   - `<API_КЛЮЧ>` — API-ключ, созданный на предыдущем шаге. Например, `AQVN1HHJReSrfo9jU3aopsXrJyfq_UHs********`.

   Также вы можете указать дополнительные параметры в соответствии с [документацией](../../modules/300-prometheus/cr.html#grafanaadditionaldatasource).

Подробнее с данной функциональностью можно ознакомиться в [документации Yandex Cloud](https://cloud.yandex.ru/ru/docs/monitoring/operations/prometheus/querying/grafana).

---
title: "Модуль cni-simple-bridge"
---

Модуль не имеет настроек.

Включается автоматически для следующих облачных провайдеров:
- [AWS](../../modules/030-cloud-provider-aws/).
- [Azure](../../modules/030-cloud-provider-azure/).
- [GCP](../../modules/030-cloud-provider-gcp/).
- [Yandex](../../modules/030-cloud-provider-yandex/).

---
title: "Управление узлами"
description: Deckhouse управляет узлами кластера Kubernetes как связанной группой, настраивает и обновляет узлы кластера, управляет масштабированием кластера в облаке и управляет Linux-пользователями на узлах.
---

## Основные функции

Управление узлами осуществляется с помощью модуля `node-manager`, основные функции которого:
1. Управление несколькими узлами как связанной группой (**NodeGroup**):
    * Возможность определить метаданные, которые наследуются всеми узлами группы.
    * Мониторинг группы узлов как единой сущности (группировка узлов на графиках по группам, группировка алертов о недоступности узлов, алерты о недоступности N узлов или N% узлов группы).
2. Систематическое прерывание работы узлов — **Chaos Monkey**. Предназначено для верификации отказоустойчивости элементов кластера и запущенных приложений.
3. Установка/обновление и настройка ПО узла (containerd, kubelet и др.), подключение узла в кластер:
    * Установка операционной системы (смотри [список поддерживаемых ОС](../../supported_versions.html#linux)) вне зависимости от типа используемой инфраструктуры (в любом облаке или на любом железе).
    * Базовая настройка операционной системы (отключение автообновления, установка необходимых пакетов, настройка параметров журналирования и т. д.).
    * Настройка nginx (и системы автоматического обновления перечня upstream’ов) для балансировки запросов от узла (kubelet) по API-серверам.
    * Установка и настройка CRI containerd и Kubernetes, включение узла в кластер.
    * Управление обновлениями узлов и их простоем (disruptions):
        * Автоматическое определение допустимой минорной версии Kubernetes группы узлов на основании ее
          настроек (указанной для группы kubernetesVersion), версии по умолчанию для всего кластера и текущей
          действительной версии control plane (не допускается обновление узлов в опережение обновления control plane).
        * Из группы одновременно производится обновление только одного узла и только если все узлы группы доступны.
        * Два варианта обновлений узлов:
            * обычные — всегда происходят автоматически;
            * требующие disruption (например, обновление ядра, смена версии containerd, значительная смена версии kubelet и пр.) — можно выбрать ручной или автоматический режим. В случае, если разрешены автоматические disruptive-обновления, перед обновлением производится drain узла (можно отключить).
    * Мониторинг состояния и прогресса обновления.
4. Масштабирование кластера.
   * Автоматическое масштабирование.

     Доступно при использовании поддерживаемых облачных провайдеров ([подробнее](#масштабирование-узлов-в-облаке)) и недоступно для статических узлов. Облачный провайдер в автоматическом режиме может создавать или удалять виртуальные машины, подключать их к кластеру или отключать.

   * Поддержание желаемого количества узлов в группе.

     Доступно как для [облачных провайдеров](#масштабирование-узлов-в-облаке), так и для статических узлов (при использовании [Cluster API Provider Static](#работа-со-статическими-узлами)).
5. Управление Linux-пользователями на узлах.

Управление узлами осуществляется через управление группой узлов (ресурс [NodeGroup](cr.html#nodegroup)), где каждая такая группа выполняет определенные для нее задачи. Примеры групп узлов по выполняемым задачам:
- группы master-узлов;
- группа узлов маршрутизации HTTP(S)-трафика (front-узлы);
- группа узлов мониторинга;
- группа узлов приложений (worker-узлы) и т. п.

Узлы в группе имеют общие параметры и настраиваются автоматически в соответствии с параметрами группы. Deckhouse масштабирует группы, добавляя, исключая и обновляя ее узлы. Допускается иметь в одной группе как облачные, так и статические узлы (серверы bare metal, виртуальные машины). Это позволяет получать узлы на физических серверах, которые могут масштабироваться за счет облачных узлов (гибридные кластеры).

Работа в [облачной инфраструктуре](#работа-с-узлами-в-поддерживаемых-облаках) осуществляется с помощью поддерживаемых облачных провайдеров. Если поддержки необходимой облачной платформы нет, возможно использование ее ресурсов в виде статических узлов.

Работа со [статическими узлами](#работа-со-статическими-узлами) (например, серверами bare metal) выполняется с помощью в провайдера CAPS (Cluster API Provider Static).

Поддерживается работа со следующими сервисами Managed Kubernetes (может быть доступен не весь функционал сервиса):
- Google Kubernetes Engine (GKE);
- Elastic Kubernetes Service (EKS).

## Типы узлов

Типы узлов, с которыми возможна работа в рамках группы узлов (ресурс [NodeGroup](cr.html#nodegroup)):
- `CloudEphemeral` — такие узлы автоматически заказываются, создаются и удаляются в настроенном облачном провайдере.
- `CloudPermanent` — отличаются тем, что их конфигурация берется не из custom resource [nodeGroup](cr.html#nodegroup), а из специального ресурса `<PROVIDER>ClusterConfiguration` (например, [AWSClusterConfiguration](../030-cloud-provider-aws/cluster_configuration.html) для AWS). Также важное отличие узлов  в том, что для применения их конфигурации необходимо выполнить `dhctl converge` (запустив инсталлятор Deckhouse). Примером CloudPermanent-узла облачного кластера является мaster-узел кластера.  
- `CloudStatic` — узел, созданный *вручную* (статический узел), размещенный в том же облаке, с которым настроена интеграция у одного из облачных провайдеров. На таком узле работает CSI и такой узел управляется `cloud-controller-manager'ом`. Объект `Node` кластера обогащается информацией о зоне и регионе, в котором работает узел. Также при удалении узла из облака соответствующий ему Node-объект будет удален в кластере.
- `Static` — статический узел, размещенный на сервере bare metal или виртуальной машине. В случае облака, такой узел не управляется `cloud-controller-manager'ом`, даже если включен один из облачных провайдеров. [Подробнее про работу со статическими узлами...](#работа-со-статическими-узлами)

## Группировка узлов и управление группами

Группировка и управление узлами как связанной группой означает, что все узлы группы будут иметь одинаковые метаданные, взятые из custom resource'а [`NodeGroup`](cr.html#nodegroup).

Для групп узлов доступен мониторинг:
- с группировкой параметров узлов на графиках группы;
- с группировкой алертов о недоступности узлов;
- с алертами о недоступности N узлов или N% узлов группы и т. п.

## Автоматическое развертывание, настройка и обновление узлов Kubernetes

Автоматическое развертывание (в *static/hybrid* — частично), настройка и дальнейшее обновление ПО работают на любых кластерах, независимо от его размещения в облаке или на bare metal.

### Развертывание узлов Kubernetes

Deckhouse автоматически разворачивает узлы кластера, выполняя следующие **идемпотентные** операции:
- Настройку и оптимизацию операционной системы для работы с containerd и Kubernetes:
  - устанавливаются требуемые пакеты из репозиториев дистрибутива;
  - настраиваются параметры работы ядра, параметры журналирования, ротация журналов и другие параметры системы.
- Установку требуемых версий containerd и kubelet, включение узла в кластер Kubernetes.
- Настройку Nginx и обновление списка upstream для балансировки запросов от узла к Kubernetes API.

### Поддержка актуального состояния узлов

Для поддержания узлов кластера в актуальном состоянии могут применяться два типа обновлений:
- **Обычные**. Такие обновления всегда применяются автоматически, и не приводят к остановке или перезагрузке узла.
- **Требующие прерывания** (disruption). Пример таких обновлений — обновление версии ядра или containerd, значительная смена версии kubelet и т. д. Для этого типа обновлений можно выбрать ручной или автоматический режим (секция параметров [disruptions](cr.html#nodegroup-v1-spec-disruptions)). В автоматическом режиме перед обновлением выполняется корректная приостановка работы узла (drain) и только после этого производится обновление.

В один момент времени производится обновление только одного узла из группы и только в том случае, когда все узлы группы доступны.

Модуль `node-manager` имеет набор встроенных метрик мониторинга, которые позволяют контролировать прогресс обновления, получать уведомления о возникающих во время обновления проблемах или о необходимости получения разрешения на обновление (ручное подтверждение обновления).

## Работа с узлами в поддерживаемых облаках

У каждого поддерживаемого облачного провайдера существует возможность автоматического заказа узлов. Для этого необходимо указать требуемые параметры для каждого узла или группы узлов.

В зависимости от провайдера этими параметрами могут быть:
- тип узлов или количество ядер процессора и объем оперативной памяти;
- размер диска;
- настройки безопасности;
- подключаемые сети и др.

Создание, запуск и подключение виртуальных машин к кластеру выполняются автоматически.

### Масштабирование узлов в облаке

Возможны два режима масштабирования узлов в группе:
- **Автоматическое масштабирование**.

  При дефиците ресурсов, наличии подов в состоянии `Pending`, в группу будут добавлены узлы. При отсутствии нагрузки на один или несколько узлов, они будут удалены из кластера. При работе автомасштабирования учитывается [приоритет](cr.html#nodegroup-v1-spec-cloudinstances-priority) группы (в первую очередь будет масштабироваться группа, у которой приоритет больше).
  
  Чтобы включить автоматическое масштабирование узлов, необходимо указать разные ненулевые значения [минимального](cr.html#nodegroup-v1-spec-cloudinstances-minperzone) и [максимального](cr.html#nodegroup-v1-spec-cloudinstances-maxperzone) количества узлов в группе.

- **Фиксированное количество узлов.**

  В этом случае Deckhouse будет поддерживать указанное количество узлов (например, заказывая новые в случае выхода из строя старых узлов).

  Чтобы указать фиксированное количество узлов в группе и отключить автоматическое масштабирование, необходимо указать одинаковые значения параметров [minPerZone](cr.html#nodegroup-v1-spec-cloudinstances-minperzone) и [maxPerZone](cr.html#nodegroup-v1-spec-cloudinstances-maxperzone).

## Работа со статическими узлами

При работе со статическими узлами функции модуля `node-manager` выполняются со следующими ограничениями:
- **Отсутствует заказ узлов.** Непосредственное выделение ресурсов (серверов bare metal, виртуальных машин, связанных ресурсов) выполняется вручную. Дальнейшая настройка ресурсов  (подключение узла к кластеру, настройка мониторинга и т.п.) выполняются полностью автоматически (аналогично узлам в облаке) или частично.
- **Отсутствует автоматическое масштабирование узлов.** Доступно поддержание в группе указанного количества узлов при использовании [Cluster API Provider Static](#cluster-api-provider-static) (параметр [staticInstances.count](cr.html#nodegroup-v1-spec-staticinstances-count)). Т.е. Deckhouse будет пытаться поддерживать указанное количество узлов в группе, очищая лишние узлы и настраивая новые при необходимости (выбирая их из ресурсов [StaticInstance](cr.html#staticinstance), находящихся в состоянии *Pending*).

Настройка/очистка узла, его подключение к кластеру и отключение могут выполняться следующими способами:
- **Вручную,** с помощью подготовленных скриптов.

  Для настройки сервера (ВМ) и ввода узла в кластер нужно загрузить и выполнить специальный bootstrap-скрипт. Такой скрипт генерируется для каждой группы статических узлов (каждого ресурса `NodeGroup`). Он находится в секрете `d8-cloud-instance-manager/manual-bootstrap-for-<ИМЯ-NODEGROUP>`. Пример добавления статического узла в кластер можно найти в [FAQ](examples.html#вручную).

  Для отключения узла кластера и очистки сервера (виртуальной машины) нужно выполнить скрипт `/var/lib/bashible/cleanup_static_node.sh`, который уже находится на каждом статическом узле. Пример отключения узла кластера и очистки сервера можно найти в [FAQ](faq.html#как-вручную-очистить-статический-узел).

- **Автоматически,** с помощью [Cluster API Provider Static](#cluster-api-provider-static).

  > Функционал доступен начиная с версии 1.54 Deckhouse и находится в стадии тестирования и активной разработки.

  Cluster API Provider Static (CAPS) подключается к серверу (ВМ) используя ресурсы [StaticInstance](cr.html#staticinstance) и [SSHCredentials](cr.html#sshcredentials), выполняет настройку, и вводит узел в кластер.

  При необходимости (например, если удален соответствующий серверу ресурс [StaticInstance](cr.html#staticinstance) или уменьшено [количество узлов группы](cr.html#nodegroup-v1-spec-staticinstances-count)), Cluster API Provider Static подключается к узлу кластера, очищает его и отключает от кластера.

### Cluster API Provider Static

> Cluster API Provider Static доступен начиная с версии 1.54 Deckhouse. Описанный функционал находится в стадии тестирования и активной разработки. Функционал и описание ресурсов могут измениться. Учитывайте это при использовании в продуктивных кластерах.

Cluster API Provider Static (CAPS), это реализация провайдера декларативного управления статическими узлами (серверами bare metal или виртуальными машинами) для проекта [Cluster API](https://cluster-api.sigs.k8s.io/) Kubernetes. По сути, CAPS это дополнительный слой абстракции к уже существующему функционалу Deckhouse по автоматической настройке и очистке статических узлов с помощью скриптов, генерируемых для каждой группы узлов (см. раздел [Работа со статическими узлами](#работа-со-статическими-узлами)).

CAPS выполняет следующие функции:
- настройка сервера bare metal (или виртуальной машины) для подключения к кластеру Kubernetes;
- подключение узла в кластер Kubernetes;
- отключение узла от кластера Kubernetes;
- очистка сервера bare metal (или виртуальной машины) после отключения узла из кластера Kubernetes.

CAPS использует следующие ресурсы (CustomResource) при работе:
- **[StaticInstance](cr.html#staticinstance).** Каждый ресурс `StaticInstance` описывает конкретный хост (сервер, ВМ), который управляется с помощью CAPS.
- **[SSHCredentials](cr.html#sshcredentials)**. Содержит данные SSH, необходимые для подключения к хосту (`SSHCredentials` указывается в параметре [credentialsRef](cr.html#staticinstance-v1alpha1-spec-credentialsref) ресурса `StaticInstance`).
- **[NodeGroup](cr.html#nodegroup)**. Секция параметров [staticInstances](cr.html#nodegroup-v1-spec-staticinstances) определяет необходимое количество узлов в группе и фильтр множества ресурсов `StaticInstance` которые могут использоваться в группе.

CAPS включается автоматически, если в NodeGroup заполнена секция параметров [staticInstances](cr.html#nodegroup-v1-spec-staticinstances). Если в `NodeGroup` секция параметров `staticInstances` не заполнена, то настройка и очистка узлов для работы в этой группе выполняется *вручную* (см. примеры [добавления статического узла в кластер](examples.html#вручную) и [очистки узла](faq.html#как-вручную-очистить-статический-узел)), а не с помощью CAPS.

Схема работы со статичными узлами при использовании Cluster API Provider Static (CAPS) ([практический пример добавления узла](examples.html#с-помощью-cluster-api-provider-static)):
1. **Подготовка ресурсов.**

   Перед тем, как отдать сервер bare metal или виртуальную машину под управление CAPS, может быть необходима предварительная подготовка, например:
   - Подготовка системы хранения, добавление точек монтирования и т. п.;
   - Установка специфических пакетов ОС. Например, установка пакета `ceph-common`, если на сервере используется тома CEPH;
   - Настройка необходимой сетевой связанности. Например, между сервером и узлами кластера;
   - Настройка доступа по SSH на сервер, создание пользователя для управления с root-доступом через `sudo`. Хорошей практикой является создание отдельного пользователя и уникальных ключей для каждого сервера.

1. **Создание ресурса [SSHCredentials](cr.html#sshcredentials).**

   В ресурсе `SSHCredentials` указываются параметры, необходимые CAPS для подключения к серверу по SSH. Один ресурс `SSHCredentials` может использоваться для подключения к нескольким серверам, но хорошей практикой является создание уникальных пользователей и ключей доступа для подключения к каждому серверу. В этом случае ресурс `SSHCredentials` также будет отдельный на каждый сервер.

1. **Создание ресурса [StaticInstance](cr.html#staticinstance).**

   На каждый сервер (ВМ) в кластере создается отдельный ресурс `StaticInstance`. В нем указан IP-адрес для подключения и ссылка на ресурс `SSHCredentials`, данные которого нужно использовать при подключении.

   Возможные состояния `StaticInstances` и связанных с ним серверов (ВМ) и узлов кластера:
   - `Pending`. Сервер не настроен, и в кластере нет соответствующего узла.
   - `Bootstraping`. Выполняется процедура настройки сервера (ВМ) и подключения узла в кластер.
   - `Running`. Сервер настроен, и в кластер добавлен соответствующий узел.
   - `Cleaning`. Выполняется процедура очистки сервера и отключение узла из кластера.

1. **Создание ресурса [NodeGroup](cr.html#nodegroup).**

   В контексте CAPS в ресурсе `NodeGroup` нужно обратить внимание на параметр [nodeType](cr.html#nodegroup-v1-spec-nodetype) (должен быть `Static`) и секцию параметров [staticInstances](cr.html#nodegroup-v1-spec-staticinstances).

   Секция параметров [staticInstances.labelSelector](cr.html#nodegroup-v1-spec-staticinstances-labelselector) определяет фильтр, по которому CAPS выбирает ресурсы `StaticInstance`, которые нужно использовать в группе. Фильтр позволяет использовать для разных групп узлов только определенные `StaticInstance`, а также позволяет использовать один `StaticInstance` в разных группах узлов. Фильтр можно не определять, чтобы использовать в группе узлов любой доступный `StaticInstance`.

   Параметр [staticInstances.count](cr.html#nodegroup-v1-spec-staticinstances-count) определяет желаемое количество узлов в группе.  При изменении параметра, CAPS начинает добавлять или удалять необходимое количество узлов, запуская этот процесс параллельно.

В соответствии с данными секции параметров [staticInstances](cr.html#nodegroup-v1-spec-staticinstances), CAPS будет пытаться поддерживать указанное (параметр [count](cr.html#nodegroup-v1-spec-staticinstances-count)) количество узлов в группе. При необходимости добавить узел в группу, CAPS выбирает соответствующий [фильтру](cr.html#nodegroup-v1-spec-staticinstances-labelselector) ресурс [StaticInstance](cr.html#staticinstance) находящийся в статусе `Pending`, настраивает сервер (ВМ) и добавляет узел в кластер. При необходимости удалить узел из группы, CAPS выбирает [StaticInstance](cr.html#staticinstance) находящийся в статусе `Running`, очищает сервер (ВМ) и удаляет узел из кластера (после чего, соответствующий `StaticInstance` переходит в состояние `Pending` и снова может быть использован).

## Пользовательские настройки на узлах

Для автоматизации действий на узлах группы предусмотрен ресурс [NodeGroupConfiguration](cr.html#nodegroupconfiguration). Ресурс позволяет выполнять на узлах bash-скрипты, в которых можно пользоваться набором команд [bashbooster](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bashbooster), а также позволяет использовать шаблонизатор [Go Template](https://pkg.go.dev/text/template). Это удобно для автоматизации таких операций, как:
- установка и настройки дополнительных пакетов ОС ([пример установки kubectl-плагина](examples.html#установка-плагина-cert-manager-для-kubectl-на-master-узлах), [пример настройки containerd с поддержкой Nvidia GPU](faq.html#как-использовать-containerd-с-поддержкой-nvidia-gpu));
- обновления ядра ОС на конкретную версию ([пример](faq.html#как-обновить-ядро-на-узлах));
- изменение параметров ОС ([пример настройки параметра sysctl](examples.html#задание-параметра-sysctl));
- сбор информации на узле и выполнение других подобных действий.

Ресурс `NodeGroupConfiguration` позволяет указывать [приоритет](cr.html#nodegroupconfiguration-v1alpha1-spec-weight) выполняемым скриптам, ограничивать их выполнение определенными [группами узлов](cr.html#nodegroupconfiguration-v1alpha1-spec-nodegroups) и [типами ОС](cr.html#nodegroupconfiguration-v1alpha1-spec-bundles).

Код скрипта указывается в параметре [content](cr.html#nodegroupconfiguration-v1alpha1-spec-content) ресурса. При создании скрипта на узле содержимое параметра `content` проходит через шаблонизатор [Go Template](https://pkg.go.dev/text/template), который позволят встроить дополнительный уровень логики при генерации скрипта. При прохождении через шаблонизатор становится доступным контекст с набором динамических переменных.

Переменные, которые доступны для использования в шаблонизаторе:
<ul>
<li><code>.cloudProvider</code> (для групп узлов с nodeType <code>CloudEphemeral</code> или <code>CloudPermanent</code>) — массив данных облачного провайдера.
{% offtopic title="Пример данных..." %}
```yaml
cloudProvider:
  instanceClassKind: OpenStackInstanceClass
  machineClassKind: OpenStackMachineClass
  openstack:
    connection:
      authURL: https://cloud.provider.com/v3/
      domainName: Default
      password: p@ssw0rd
      region: region2
      tenantName: mytenantname
      username: mytenantusername
    externalNetworkNames:
    - public
    instances:
      imageName: ubuntu-22-04-cloud-amd64
      mainNetwork: kube
      securityGroups:
      - kube
      sshKeyPairName: kube
    internalNetworkNames:
    - kube
    podNetworkMode: DirectRoutingWithPortSecurityEnabled
  region: region2
  type: openstack
  zones:
  - nova
```
{% endofftopic %}</li>
<li><code>.cri</code> — используемый CRI (с версии Deckhouse 1.49 используется только <code>Containerd</code>).</li>
<li><code>.kubernetesVersion</code> — используемая версия Kubernetes.</li>
<li><code>.nodeUsers</code> — массив данных о пользователях узла, добавленных через ресурс <a href="cr.html#nodeuser">NodeUser</a>.
{% offtopic title="Пример данных..." %}
```yaml
nodeUsers:
- name: user1
  spec:
    isSudoer: true
    nodeGroups:
    - '*'
    passwordHash: PASSWORD_HASH
    sshPublicKey: SSH_PUBLIC_KEY
    uid: 1050
```
{% endofftopic %}
</li>
<li><code>.nodeGroup</code> — массив данных группы узлов.
{% offtopic title="Пример данных..." %}
```yaml
nodeGroup:
  cri:
    type: Containerd
  disruptions:
    approvalMode: Automatic
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: "Off"
  kubernetesVersion: "1.27"
  manualRolloutID: ""
  name: master
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
  nodeType: CloudPermanent
  updateEpoch: "1699879470"
```
{% endofftopic %}</li>
</ul>

{% raw %}
Пример использования переменных в шаблонизаторе:

```shell
{{- range .nodeUsers }}
echo 'Tuning environment for user {{ .name }}'
# Some code for tuning user environment
{{- end }}
```

Пример использования команд bashbooster:

```shell
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
}
```

{% endraw %}
Ход выполнения скриптов можно увидеть на узле в журнале сервиса bashible (`journalctl -u bashible.service`). Сами скрипты находятся на узле в директории `/var/lib/bashible/bundle_steps/`.

## Chaos Monkey

Инструмент (включается у каждой из `NodeGroup` отдельно), позволяющий систематически вызывать случайные прерывания работы узлов. Предназначен для проверки элементов кластера, приложений и инфраструктурных компонентов на реальную работу отказоустойчивости.

---
title: "Модуль terraform-manager"
description: Описание модуля terraform-manager Deckhouse. Модуль следит за приведением объектов в кластере к состоянию, описанному в Terraform state.   
---

Модуль предоставляет инструменты для работы с состоянием Terraform'а в кластере Kubernetes.

* Модуль состоит из двух частей:
  * `terraform-auto-converger` — проверяет состояние Terraform'а и применяет недеструктивные изменения;
  * `terraform-state-exporter` — проверяет состояние Terraform'а и экспортирует метрики кластера.

* Модуль включен по умолчанию, если в кластере есть Secret'ы:
  * `kube-system/d8-provider-cluster-configuration`;
  * `d8-system/d8-cluster-terraform-state`.

---
title: "Модуль snapshot-controller"
---

Этот модуль включает поддержку снапшотов для совместимых CSI-драйверов в кластере Kubernetes.

CSI-драйверы в Deckhouse, которые поддерживают снапшоты:
- [cloud-provider-openstack](../030-cloud-provider-openstack/);
- [cloud-provider-vsphere](../030-cloud-provider-vsphere/);
- [ceph-csi](../031-ceph-csi/);
- [cloud-provider-aws](../030-cloud-provider-aws/);
- [cloud-provider-azure](../030-cloud-provider-azure/);
- [cloud-provider-gcp](../030-cloud-provider-gcp/);
- [linstor](../041-linstor/).

---
title: "Модуль ingress-nginx"
---

Устанавливает и управляет [NGINX Ingress controller](https://github.com/kubernetes/ingress-nginx) с помощью Custom Resources. Если узлов для размещения Ingress-контроллера больше одного, он устанавливается в отказоустойчивом режиме и учитывает все особенности реализации инфраструктуры облаков и bare metal, а также кластеров Kubernetes различных типов.

Поддерживает запуск и раздельное конфигурирование одновременно нескольких NGINX Ingress controller'ов — один **основной** и сколько угодно **дополнительных**. Например, это позволяет отделять внешние и intranet Ingress-ресурсы приложений.

## Варианты терминирования трафика

Трафик к nginx-ingress может быть отправлен несколькими способами:
- напрямую без внешнего балансировщика;
- через внешний LoadBalancer, в том числе поддерживаются:
  - Qrator,
  - Cloudflare,
  - AWS LB,
  - GCE LB,
  - ACS LB,
  - Yandex LB,
  - OpenStack LB.

## Терминация HTTPS

Модуль позволяет управлять для каждого из NGINX Ingress controller'а политиками безопасности HTTPS, в частности:
- параметрами HSTS;
- набором доступных версий SSL/TLS и протоколов шифрования.

Также модуль интегрирован с модулем [cert-manager](../../modules/101-cert-manager/), при взаимодействии с которым возможны автоматический заказ SSL-сертификатов и их дальнейшее использование NGINX Ingress controller'ами.

## Мониторинг и статистика

В нашей реализации `ingress-nginx` добавлена система сбора статистики в Prometheus с множеством метрик:
- по длительности времени всего ответа и апстрима отдельно;
- кодам ответа;
- количеству повторов запросов (retry);
- размерам запроса и ответа;
- методам запросов;
- типам `content-type`;
- географии распределения запросов и т. д.

Данные доступны в нескольких разрезах:
- по `namespace`;
- `vhost`;
- `ingress`-ресурсу;
- `location` (в nginx).

Все графики собраны в виде удобных досок в Grafana, при этом есть возможность drill-down'а по графикам: при просмотре, например, статистики в разрезе namespace есть возможность, нажав на ссылку на dashboard в Grafana, углубиться в статистику по `vhosts` в этом `namespace` и т. д.

## Статистика

### Основные принципы сбора статистики

1. На каждый запрос на стадии `log_by_lua_block` вызывается наш модуль, который рассчитывает необходимые данные и складывает их в буфер (у каждого nginx worker'а свой буфер).
2. На стадии `init_by_lua_block` для каждого nginx worker'а запускается процесс, который раз в секунду асинхронно отправляет данные в формате `protobuf` через TCP socket в `protobuf_exporter` (наша собственная разработка).
3. `protobuf_exporter` запущен sidecar-контейнером в поде с ingress-controller'ом, принимает сообщения в формате `protobuf`, разбирает, агрегирует их по установленным нами правилам и экспортирует в формате для Prometheus.
4. Prometheus каждые 30 секунд scrape'ает как сам ingress-controller (там есть небольшое количество нужных нам метрик), так и protobuf_exporter, на основании этих данных все и работает!

### Какая статистика собирается и как она представлена

У всех собираемых метрик есть служебные лейблы, позволяющие идентифицировать экземпляр контроллера: `controller`, `app`, `instance` и `endpoint` (они видны в `/prometheus/targets`).

* Все метрики (кроме geo), экспортируемые protobuf_exporter'ом, представлены в трех уровнях детализации:
  * `ingress_nginx_overall_*` — «вид с вертолета», у всех метрик есть лейблы `namespace`, `vhost` и `content_kind`;
  * `ingress_nginx_detail_*` — кроме лейблов уровня overall, добавляются `ingress`, `service`, `service_port` и `location`;
  * `ingress_nginx_detail_backend_*` — ограниченная часть данных, собирается в разрезе по бэкендам. У этих метрик, кроме лейблов уровня detail, добавляется лейбл `pod_ip`.

* Для уровней overall и detail собираются следующие метрики:
  * `*_requests_total` — counter количества запросов (дополнительные лейблы — `scheme`, `method`);
  * `*_responses_total` — counter количества ответов (дополнительный лейбл — `status`);
  * `*_request_seconds_{sum,count,bucket}` — histogram времени ответа;
  * `*_bytes_received_{sum,count,bucket}` — histogram размера запроса;
  * `*_bytes_sent_{sum,count,bucket}` — histogram размера ответа;
  * `*_upstream_response_seconds_{sum,count,bucket}` — histogram времени ответа upstream'а (используется сумма времен ответов всех upstream'ов, если их было несколько);
  * `*_lowres_upstream_response_seconds_{sum,count,bucket}` — то же самое, что и предыдущая метрика, только с меньшей детализацией (подходит для визуализации, но не подходит для расчета quantile);
  * `*_upstream_retries_{count,sum}` — количество запросов, при обработке которых были retry бэкендов, и сумма retry'ев.

* Для уровня overall собираются следующие метрики:
  * `*_geohash_total` — counter количества запросов с определенным geohash (дополнительные лейблы — `geohash`, `place`).

* Для уровня detail_backend собираются следующие метрики:
  * `*_lowres_upstream_response_seconds` — то же самое, что аналогичная метрика для overall и detail;
  * `*_responses_total` — counter количества ответов (дополнительный лейбл — `status_class`, а не просто `status`);
  * `*_upstream_bytes_received_sum` — counter суммы размеров ответов бэкенда.

---
title: "Модуль pod-reloader"
---

Модуль создан на основе [Reloader](https://github.com/stakater/Reloader).
Он предоставляет возможность автоматически произвести rollout в случае изменения ConfigMap или Secret.
Для управления используются аннотации. Модуль запускается на **системных** узлах.

> **Важно!** У Reloader отсутствует отказоустойчивость.

В этом документе описаны основные аннотации. Вы можете найти больше примеров в разделе [Примеры](examples.html) документации.

| Аннотация                                    | Ресурс                             | Описание                                                                                                                                                                 | Примеры значений                              |
| -------------------------------------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------- |
| `pod-reloader.deckhouse.io/auto`             | Deployment, Daemonset, Statefulset | В случае изменения в связанных, то есть примонтированных или использованных как переменные окружения, ConfigMap'ах или Secret'ах произойдет перезапуск подов этого контроллера | `"true"`, `"false"`  |
| `pod-reloader.deckhouse.io/search`           | Deployment, Daemonset, Statefulset | В случае наличия этой аннотации перезапуск будет производиться исключительно при изменении ConfigMap'ов или Secret'ов с аннотацией `pod-reloader.deckhouse.io/match: "true"` | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/configmap-reload` | Deployment, Daemonset, Statefulset | Указать список ConfigMap'ов, от которых зависит контроллер                                                                                                                   | `"some-cm"`, `"some-cm1,some-cm2"` |
| `pod-reloader.deckhouse.io/secret-reload`    | Deployment, Daemonset, Statefulset | Указать список Secret'ов, от которых зависит контроллер                                                                                                                      | `"some-secret"`, `"some-secret1,some-secret2"` |
| `pod-reloader.deckhouse.io/match`            | Secret, Configmap                  | Аннотация, по которой из связанных ресурсов выбираются те, за изменениями которых мы следим                                                                               | `"true"`, `"false"` |

**Важно** Аннотация `pod-reloader.deckhouse.io/search` не может быть использована вместе с `pod-reloader.deckhouse.io/auto: "true"`, так как Reloader будет игнорировать `pod-reloader.deckhouse.io/search` и `pod-reloader.deckhouse.io/match`. Для корректной работы установите аннотации `pod-reloader.deckhouse.io/auto` значение `"false"` или удалите ее.

**Важно** Аннотации `pod-reloader.deckhouse.io/configmap-reload` и `pod-reloader.deckhouse.io/secret-reload` не могут быть использованы вместе с `pod-reloader.deckhouse.io/auto: "true"`, так как Reloader будет игнорировать `pod-reloader.deckhouse.io/search` и `pod-reloader.deckhouse.io/match`. Для корректной работы установите аннотации `pod-reloader.deckhouse.io/auto` значение `"false"` или удалите ее.

---
title: "Модуль chrony"
---

Обеспечивает синхронизацию времени на всех узлах кластера с помощью модуля [chrony](https://chrony.tuxfamily.org/).

---
title: "Модуль delivery"
webIfaces:
- name: argocd
---

Модуль предоставляет возможность использовать инструмент для Continuous Deployment — [Argo CD](https://argo-cd.readthedocs.io/en/stable/).

Рекомендуется использовать модуль `delivery` в связке с [werf bundles](https://ru.werf.io/documentation/v1.2/advanced/bundles.html).

---
title: "Модуль namespace-configurator"
---

Позволяет автоматически управлять аннотациями и label'ами на namespace'ах.

Модуль полезен тем, что помогает автоматически включать новые namespace'ы в мониторинг посредством добавления лейбла `extended-monitoring.deckhouse.io/enabled=true`.

### Как работает

Модуль следит за изменениями namespace и своей конфигурации:
* Всем namespace'ам, попадающим под шаблон `includeNames` и не попадающим под шаблон `excludeNames`, будут назначены соответствующие label'ы и аннотации из конфигурации.
* При изменении конфигурации модуля соответствующие label'ы и аннотации на namespace'ах будут переназначены согласно конфигурациии.

### Что нужно настроить?

Необходимо перечислить список желаемых label'ов и аннотаций, а также список шаблонов поиска namespace в конфигурации модуля.


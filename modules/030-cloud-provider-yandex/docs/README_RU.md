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

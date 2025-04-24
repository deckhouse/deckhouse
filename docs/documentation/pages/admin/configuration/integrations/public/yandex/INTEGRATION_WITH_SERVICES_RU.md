---
title: Интеграция с службами Yandex Cloud
permalink: ru/admin/integrations/public/yandex/yandex-services.html
lang: ru
---

Deckhouse поддерживает нативную интеграцию с несколькими облачными сервисами Yandex Cloud. Это позволяет подключать внешний мониторинг, безопасно работать с секретами и автоматизировать синхронизацию данных между инфраструктурой и кластером.

В данном разделе описаны шаги по настройке следующих интеграций:

- с Yandex Lockbox через External Secrets Operator;
- с Yandex Managed Service for Prometheus.

## Интеграция с Yandex Lockbox

Deckhouse поддерживает интеграцию с Yandex Lockbox с помощью оператора External Secrets Operator (ESO). Это позволяет автоматически синхронизировать секреты из Lockbox с секретами Kubernetes в кластере.

Для интеграции выполните следующие шаги:

1. Создайте сервисный аккаунт:

   ```console
   yc iam service-account create --name eso-service-account
   ```

1. Создайте авторизованный ключ:

   ```console
   yc iam key create --service-account-name eso-service-account --output authorized-key.json
   ```

1. Назначьте роли для доступа к Lockbox. Замените `<folder_id>` на ваш фактический идентификатор каталога:

   ```console
   folder_id=<идентификатор каталога>

   yc resource-manager folder add-access-binding --id=${folder_id} \
     --service-account-name eso-service-account --role lockbox.editor

   yc resource-manager folder add-access-binding --id=${folder_id} \
     --service-account-name eso-service-account --role lockbox.payloadViewer

   yc resource-manager folder add-access-binding --id=${folder_id} \
     --service-account-name eso-service-account --role kms.keys.encrypterDecrypter
   ```

1. Установите External Secrets Operator:

   - Скачайте и распакуйте Helm-чарт:

     ```console
     helm pull oci://cr.yandex/yc-marketplace/yandex-cloud/external-secrets/chart/external-secrets \
       --version 0.5.5 \
       --untar
     ```

   - Установите Helm-чарт с указанием ключа:

     ```console
     helm install -n external-secrets --create-namespace \
       --set-file auth.json=authorized-key.json \
       external-secrets ./external-secrets/
     ```

     При необходимости задайте `nodeSelector`, `tolerations` и другие параметры через `./external-secrets/values.yaml`.

1. Создайте `SecretStore`:

   ```yaml
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
             key: key
   ```

   `sa-creds` — секрет, содержащий ключ (`authorized-key.json`), созданный при установке чарта.
   `key` — имя поля в `.data,` в котором находится содержимое ключа.

1. Проверьте работоспособность с помощью команд:

   ```console
   kubectl -n external-secrets get po

   kubectl -n external-secrets get secretstores.external-secrets.io
   ```

   Пример корректного вывода:

   ```console
   NAME           AGE   STATUS
   secret-store   69m   Valid
   ```

1. Создайте объект `ExternalSecret`:

   - Создайте секрет в Lockbox с параметрами:
     - Имя: `lockbox-secret`
     - Ключ: `password`
     - Значение: `p@$$w0rd`

   - Создайте объект `ExternalSecret`:

     ```yaml
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
           property: password
     ```

   - Проверьте результат:

     ```console
     kubectl -n external-secrets get secret k8s-secret -ojson | jq -r '.data.password' | base64 -d
     ```

     Вывод должен содержать значение `p@$$w0rd`.

## Интеграция с Yandex Managed Service for Prometheus

Deckhouse позволяет использовать Yandex Managed Service for Prometheus как внешнее хранилище для метрик.

Для запись метрик (PrometheusRemoteWrite) выполните следующие шаги:

- Создайте сервисный аккаунт с ролью `monitoring.editor`.
- Сгенерируйте API-ключ.
- Примените манифест:

  ```yaml
  apiVersion: deckhouse.io/v1
  kind: PrometheusRemoteWrite
  metadata:
    name: yc-remote-write
  spec:
    url: <URL_ЗАПИСИ_МЕТРИК>
    bearerToken: <API_КЛЮЧ>
  ```

  > `URL` и `API_КЛЮЧ` можно получить в интерфейсе Yandex Cloud: Monitoring → Prometheus → Запись метрик.

Для чтения метрик через Grafana:

- Создайте сервисный аккаунт с ролью `monitoring.viewer`.
- Сгенерируйте API-ключ.
- Примените манифест:

  ```yaml
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
  ```

  `URL` можно получить в интерфейсе Yandex Monitoring → Prometheus → Чтение метрик.

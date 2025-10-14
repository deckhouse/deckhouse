---
title: "The secrets-store-integration module: примеры"
description: Использование модуля secrets-store-integration.
---

## Настройка модуля для работы c Deckhouse Stronghold

Для автоматической настройки работы модуля secrets-store-integration в связке с модулем [Deckhouse Stronghold](/modules/stronghold/) потребуется ранее [включенный](/modules/stronghold/usage.html#включение-модуля) и настроенный Stronghold.

Далее достаточно применить следующий ресурс:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: secrets-store-integration
spec:
  enabled: true
  version: 1
```

Параметр [connectionConfiguration](configuration.html#parameters-connectionconfiguration) можно опустить, поскольку он стоит в значении `DiscoverLocalStronghold` по умолчанию.

## Настройка модуля для работы с внешним хранилищем

Для работы модуля требуется предварительно настроенное хранилище секретов, совместимое с HashiCorp Vault. В хранилище предварительно должен быть настроен путь аутентификации. Пример настройки хранилища секретов [ниже](#подготовка-тестового-окружения).

Чтобы убедиться, что каждый API запрос зашифрован, послан и отвечен правильным адресатом, потребуется валидный публичный сертификат Certificate Authority, который используется хранилищем секретов. Такой публичный сертификат CA в PEM-формате необходимо использовать в качестве переменной `caCert` в конфигурации модуля.

Пример конфигурации модуля для использования Vault-совместимого хранилища секретов, запущенного по адресу «secretstoreexample.com» на TLS-порту по умолчанию - 443 TLS:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
 name: secrets-store-integration
spec:
 version: 1
 enabled: true
 settings:
   connection:
     url: "https://secretstoreexample.com"
     authPath: "main-kube"
     caCert: |
       -----BEGIN CERTIFICATE-----
       MIIFoTCCA4mgAwIBAgIUX9kFz7OxlBlALMEj8WsegZloXTowDQYJKoZIhvcNAQEL
       ................................................................
       WoR9b11eYfyrnKCYoSqBoi2dwkCkV1a0GN9vStwiBnKnAmV3B8B5yMnSjmp+42gt
       o2SYzqM=
       -----END CERTIFICATE-----
   connectionConfiguration: Manual
```

**Крайне рекомендуется задавать переменную `caCert`. Если она не задана, будет использовано содержимое системного ca-certificates.**

## Подготовка тестового окружения

{% alert level="info" %}
Для выполнения дальнейших команд необходим адрес и токен с правами root от Stronghold.
Такой токен можно получить во время инициализации нового secrets store.

Далее в командах будет подразумеваться что данные настойки указаны в переменных окружения.
```bash
export VAULT_TOKEN=xxxxxxxxxxx
export VAULT_ADDR=https://secretstoreexample.com
```
{% endalert %}

> В этом руководстве мы приводим два вида примерных команд:
>   * команда с использованием [мультитула d8](#скачать-мультитул-d8-для-команд-stronghold);
>   * команда с использованием curl для выполнения прямых запросов в API secrets store.

Для использования инструкций по инжектированию секретов из примеров ниже вам понадобится:

1. Создать в Stronghold секрет типа kv2 по пути `demo-kv/myapp-secret` и поместить туда значения `DB_USER` и `DB_PASS`.
2. При необходимости добавляем путь аутентификации (authPath) для аутентификации и авторизации в Stronghold с помощью Kubernetes API удалённого кластера
3. Создать в Stronghold политику `myapp-ro-policy`, разрешающую чтение секретов по пути `demo-kv/myapp-secret`.
4. Создать в Stronghold роль `myapp-role` для сервис-аккаунта `myapp-sa` в неймспейсе `myapp-namespace` и привязать к ней созданную ранее политику.
5. Создать в кластере неймспейс `myapp-namespace`.
6. Создать в созданном неймспейсе сервис-аккаунт `myapp-sa`.

Пример команд, с помощью которых можно подготовить окружение

* Включим и создадим Key-Value хранилище:

  ```bash
  stronghold secrets enable -path=demo-kv -version=2 kv
  ```
  Команда с использованием curl:

  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data '{"type":"kv","options":{"version":"2"}}' \
    ${VAULT_ADDR}/v1/sys/mounts/demo-kv
  ```

* Зададим имя пользователя и пароль базы данных в качестве значения секрета:

  ```bash
  stronghold kv put demo-kv/myapp-secret DB_USER="username" DB_PASS="secret-password"
  ```
  Команда с использованием curl:

  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request PUT \
    --data '{"data":{"DB_USER":"username","DB_PASS":"secret-password"}}' \
    ${VAULT_ADDR}/v1/demo-kv/data/myapp-secret
  ```

* Проверим, правильно ли записались секреты:

  ```bash
  stronghold kv get demo-kv/myapp-secret
  ```

  Команда с использованием curl:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    ${VAULT_ADDR}/v1/demo-kv/data/myapp-secret
  ```

* По умолчанию метод аутентификации в Stronghold через Kubernetes API кластера, на котором запущен сам Stronghold, – включён и настроен под именем `kubernetes_local`. Если требуется настроить доступ через удалённые кластера, задаём путь аутентификации (`authPath`) и включаем аутентификацию и авторизацию в Stronghold с помощью Kubernetes API для каждого кластера:

  ```bash
  stronghold auth enable -path=remote-kube-1 kubernetes
  ```
  Команда с использованием curl:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data '{"type":"kubernetes"}' \
    ${VAULT_ADDR}/v1/sys/auth/remote-kube-1
  ```

* Задаём адрес Kubernetes API для каждого кластера:

  ```bash
  stronghold write auth/remote-kube-1/config \
    kubernetes_host="https://api.kube.my-deckhouse.com"
  ```
  Команда с использованием curl:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request PUT \
    --data '{"kubernetes_host":"https://api.kube.my-deckhouse.com"}' \
    ${VAULT_ADDR}/v1/auth/remote-kube-1/config
  ```

* Создаём в Stronghold политику с названием `myapp-ro-policy`, разрешающую чтение секрета `myapp-secret`:

  ```bash
  stronghold policy write myapp-ro-policy - <<EOF
  path "demo-kv/data/myapp-secret" {
    capabilities = ["read"]
  }
  EOF
  ```
  Команда с использованием curl:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request PUT \
    --data '{"policy":"path \"demo-kv/data/myapp-secret\" {\n capabilities = [\"read\"]\n}\n"}' \
    ${VAULT_ADDR}/v1/sys/policies/acl/myapp-ro-policy
  ```


* Создаём роль, состоящую из названия пространства имён и политики. Связываем её с ServiceAccount `myapp-sa` из пространства имён `myapp-namespace` и политикой `myapp-ro-policy`:

  {% alert level="danger" %}
  **Важно!**
  Помимо настроек со стороны Stronghold, вы должны настроить разрешения авторизации используемых `serviceAccount` в кластере kubernetes.
  Подробности в пункте [ниже](#как-разрешить-serviceaccount-авторизоваться-в-stronghold)
  {% endalert %}

  ```bash
  stronghold write auth/kubernetes_local/role/myapp-role \
      bound_service_account_names=myapp-sa \
      bound_service_account_namespaces=myapp-namespace \
      policies=myapp-ro-policy \
      ttl=10m
  ```
  Команда с использованием curl:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request PUT \
    --data '{"bound_service_account_names":"myapp-sa","bound_service_account_namespaces":"myapp-namespace","policies":"myapp-ro-policy","ttl":"10m"}' \
    ${VAULT_ADDR}/v1/auth/kubernetes_local/role/myapp-role
  ```


* Повторяем то же самое для остальных кластеров, указав другой путь аутентификации:

  ```bash
  stronghold write auth/remote-kube-1/role/myapp-role \
      bound_service_account_names=myapp-sa \
      bound_service_account_namespaces=myapp-namespace \
      policies=myapp-ro-policy \
      ttl=10m
  ```
  Команда с использованием curl:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request PUT \
    --data '{"bound_service_account_names":"myapp-sa","bound_service_account_namespaces":"myapp-namespace","policies":"myapp-ro-policy","ttl":"10m"}' \
    ${VAULT_ADDR}/v1/auth/remote-kube-1/role/myapp-role
  ```


{% alert level="info" %}
**Важно!**
Рекомендованное значение TTL для токена Kubernetes составляет 10m.
{% endalert %}

Эти настройки позволяют любому поду из пространства имён `myapp-namespace` из обоих K8s-кластеров, который использует ServiceAccount `myapp-sa`, аутентифицироваться и авторизоваться в Stronghold для чтения секретов согласно политике `myapp-ro-policy`.

* Создадим namespace и ServiceAccount в указанном namespace:
  ```bash
  kubectl create namespace myapp-namespace
  kubectl -n myapp-namespace create serviceaccount myapp-sa
  ```

## Как разрешить ServiceAccount авторизоваться в Stronghold?

Для авторизации в Stronghold Pod использует токен, сгенерированный для своего ServiceAccount'а. Для того чтобы Stronghold мог проверить валидность предоставляемых данных `ServiceAccount`, используемый сервисом Stronghold должен иметь разрешение на действия `get`, `list` и `watch`  для endpoints `tokenreviews.authentication.k8s.io` и `subjectaccessreviews.authorization.k8s.io`. Для этого также можно использовать clusterRole `system:auth-delegator`.

Stronghold может использовать различные авторизационные данные для осуществления запросов в API Kubernetes:
1. Использовать токен приложения, которое пытается авторизоваться в Stronghold. В этом случае для каждого сервиса, авторизующегося в Stronghold, требуется в используемом ServiceAccount'е иметь clusterRole `system:auth-delegator` (либо права на API представленные выше).
2. Использовать статичный токен отдельно созданного специально для Stronghold `ServiceAccount` у которого имеются необходимые права.

## Инжектирование переменных окружения

### Как работает

При включении модуля в кластере появляется mutating-webhook, который при наличии у пода аннотации `secrets-store.deckhouse.io/role` изменяет манифест пода, добавляя туда инжектор. В измененном поде добавляется инит-контейнер, который помещает из служебного образа собранный статически бинарный файл-инжектор в общую для всех контейнеров пода временную директорию. В остальных контейнерах оригинальные команды запуска заменяются на запуск файла-инжектора, который получает из Vault-совместимого хранилища необходимые данные, используя для подключения сервисный аккаунт приложения, помещает эти переменные в ENV процесса, после чего выполняет системный вызов execve, запуская оригинальную команду.

Если в манифесте пода у контейнера отсутствует команда запуска, то выполняется извлечение манифеста образа из хранилица образов (реджистри), и команда извлекается из него.
Для получения манифеста из приватного хранилища образов используются заданные в манифесте пода учетные данные из `imagePullSecrets`.

Доступные аннотации, позволяющие изменять поведение инжектора
| Аннотация                                        | Умолчание |  Назначение |
|--------------------------------------------------|-----------|-------------|
|secrets-store.deckhouse.io/addr                   | из модуля | Адрес хранилища секретов в формате https://stronghold.mycompany.tld:8200 |
|secrets-store.deckhouse.io/auth-path              | из модуля | Путь, который следует использовать при аутентификации |
|secrets-store.deckhouse.io/namespace              | из модуля | Пространство имен, которое будет использоваться для подключения к хранилищу |
|secrets-store.deckhouse.io/role                   |           | Роль, с которой будет выполнено подключение к хранилищу секретов |
|secrets-store.deckhouse.io/env-from-path          |           | Строка, содержащя список путей к секретам в хранилище через запятую, из которых будут извлечены все ключи и помещены в environment. Приоритет имеют ключи, которые находятся в списке ближе к концу. |
|secrets-store.deckhouse.io/ignore-missing-secrets | false     | Запускает оригинальное приложение в случае ошибки получения секрета из хранилища |
|secrets-store.deckhouse.io/client-timeout         | 10s       | Таймаут операции получения секретов |
|secrets-store.deckhouse.io/mutate-probes          | false     | Инжектирует переменные окружения в пробы |
|secrets-store.deckhouse.io/log-level              | info      | Уровень логирования |
|secrets-store.deckhouse.io/enable-json-log        | false     | Формат логов, строка или json |
|secrets-store.deckhouse.io/skip-mutate-containers |           | Список имен контейнеров через пробел, к которым не будет применятся инжектирование |

Используя инжектор вы сможете задавать в манифестах пода вместо значений env-шаблоны, которые будут заменяться на этапе запуска контейнера на значения из хранилища.

{% alert level="info" %}
**Примечание**
Подключение переменных из ветки хранилища имеет более высокий приоритет, чем подключение явно заданных переменных из хранилища. Это значит, что при использовании одновременно аннотации `secrets-store.deckhouse.io/env-from-path` с путем до секрета, который содержит, к примеру, ключ `MY_SECRET`, и переменную окружения в манифесте с тем же именем:
```yaml
env:
  - name: MY_SECRET
    value: secrets-store:demo-kv/data/myapp-secret#password
```
в переменную окружения `MY_SECRET` внутри контейнера запишется значение секрета из **аннотации**.
{% endalert %}

Пример: извлечь из Vault-совместимого хранилища ключ `DB_PASS` из kv2-секрета по адресу `demo-kv/myapp-secret`:

```yaml
env:
  - name: PASSWORD
    value: secrets-store:demo-kv/data/myapp-secret#DB_PASS
```

Пример: извлечь из Vault-совместимого хранилища ключ `DB_PASS` версии `4` из kv2-секрета по адресу `demo-kv/myapp-secret`:

```yaml
env:
  - name: PASSWORD
    value: secrets-store:demo-kv/data/myapp-secret#DB_PASS#4
```

Шаблон может также находиться в ConfigMap или в Secret и быть подключен с помощью `envFrom`
```yaml
envFrom:
  - secretRef:
      name: app-secret-env
  - configMapRef:
      name: app-env

```
Инжектирование реальных секретов из Vault-совместимого хранилища выполнится только на этапе запуска приложения, в Secret и ConfigMap будут находиться шаблоны.


### Подключение переменных из ветки хранилища (всех ключей одного секрета)

Создадим под с названием `myapp1`, который подключит все переменные из хранилища по пути `demo-kv/data/myapp-secret`:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: myapp1
  namespace: myapp-namespace
  annotations:
    secrets-store.deckhouse.io/role: "myapp-role"
    secrets-store.deckhouse.io/env-from-path: demo-kv/data/common-secret,demo-kv/data/myapp-secret
spec:
  serviceAccountName: myapp-sa
  containers:
  - image: alpine:3.20
    name: myapp
    command:
    - sh
    - -c
    - while printenv; do sleep 5; done
```

Применим его:

```bash
kubectl create --filename myapp1.yaml
```

Проверим логи пода после его запуска, мы должны увидеть все переменные из `demo-kv/data/myapp-secret`:

```bash
kubectl -n myapp-namespace logs myapp1
```

Удалим под

```bash
kubectl -n myapp-namespace delete pod myapp1 --force
```

### Подключение явно заданных переменных из хранилища

Создадим тестовый под с названием `myapp2`, который подключит требуемые переменные из хранилища по шаблону:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: myapp2
  namespace: myapp-namespace
  annotations:
    secrets-store.deckhouse.io/role: "myapp-role"
spec:
  serviceAccountName: myapp-sa
  containers:
  - image: alpine:3.20
    env:
    - name: DB_USER
      value: secrets-store:demo-kv/data/myapp-secret#DB_USER
    - name: DB_PASS
      value: secrets-store:demo-kv/data/myapp-secret#DB_PASS
    name: myapp
    command:
    - sh
    - -c
    - while printenv; do sleep 5; done
```

Применим его:

```bash
kubectl create --filename myapp2.yaml
```

Проверим логи пода после его запуска, мы должны увидеть переменные из `demo-kv/data/myapp-secret`:

```bash
kubectl -n myapp-namespace logs myapp2
```

Удалим под

```bash
kubectl -n myapp-namespace delete pod myapp2 --force
```

## Монтирование секрета из хранилища в качестве файла в контейнер

Для доставки секретов в приложение нужно использовать CustomResource `SecretStoreImport`.

В этом примере используем уже созданные ServiceAccount `myapp-sa` и namespace `myapp-namespace` из шага [Подготовка тестового окружения](#подготовка-тестового-окружения)

Создайте в кластере CustomResource _SecretsStoreImport_ с названием `myapp-ssi`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecretsStoreImport
metadata:
  name: myapp-ssi
  namespace: myapp-namespace
spec:
  type: CSI
  role: myapp-role
  files:
    - name: "db-password"
      source:
        path: "demo-kv/data/myapp-secret"
        key: "DB_PASS"
```

Создайте в кластере тестовый под с названием `myapp3`, который подключит требуемые переменные из хранилища в виде файла:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: myapp3
  namespace: myapp-namespace
spec:
  serviceAccountName: myapp-sa
  containers:
  - image: alpine:3.20
    name: myapp
    command:
    - sh
    - -c
    - while cat /mnt/secrets/db-password; do echo; sleep 5; done
    name: backend
    volumeMounts:
    - name: secrets
      mountPath: "/mnt/secrets"
  volumes:
  - name: secrets
    csi:
      driver: secrets-store.csi.deckhouse.io
      volumeAttributes:
        secretsStoreImport: "myapp-ssi"
```
После применения этих ресурсов будет создан под, внутри которого запустится контейнер с названием `backend`. В файловой системе этого контейнера будет каталог `/mnt/secrets` с примонтированным к нему томом `secrets`. Внутри этого каталога будет лежать файл `db-password` с паролем от базы данных (`DB_PASS`) из хранилища ключ-значение Stronghold.

Проверьте логи пода после его запуска (должно выводиться содержимое файла `/mnt/secrets/db-password`):
```bash
kubectl -n myapp-namespace logs myapp3
```

Удалите под:

```bash
kubectl -n myapp-namespace delete pod myapp3 --force
```
### Доставка бинарных файлов в контейнер

Бывают ситуации, когда вам требуется доставить бинарный файл в контейнер. Это может быть JKS контейнер с ключами,
или keytab для Kerberos аутентификации.
В этом случае вы можете закодировать бинарный файл через base64 и поместить в хранилище секретов, а при извлечении
CSI-драйвер раскодирует ваши данные, и поместит в контейнер бинарный файл. Для этого нужно установить параметр
`decodeBase64` в `true` для соответствующего файла.
Если декодирование произвести не получится (например, в хранилище находится невалидный base64), контейнер не будет создан.

Пример:

Помещаем файл в хранилище

```bash
d8 stronghold kv put demo-kv/myapp-secret keytab=$(cat /path/to/keytab_file | base64 -w0)
```

Манифест SecretsStoreImport

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecretsStoreImport
metadata:
  name: myapp-ssi
  namespace: myapp-namespace
spec:
  type: CSI
  role: myapp-role
  files:
    - name: "keytab"
      decodeBase64: true
      source:
        path: "demo-kv/data/myapp-secret"
        key: "keytab"
```

В этом случае в контейнере будет создан бинарный файл с именем `keytab`

### Функция авторотации

Функция авторотации секретов в модуле secret-store-integration включена по умолчанию. Каждые две минуты модуль опрашивает Stronghold и синхронизирует секреты в примонтированном файле в случае его изменения.

Есть два варианта следить за изменениями файла с секретом в поде. Первый - следить за временем изменения примонтированного файла, реагируя на его изменение. Второй - использовать inotify API, который предоставляет механизм для подписки на события файловой системы. Inotify является частью ядра Linux. После обнаружения изменений есть большое количество вариантов реагирования на событие изменения в зависимости от используемой архитектуры приложения и используемого языка программирования. Самый простой — заставить K8s перезапустить под, перестав отвечать на liveness-пробу.

Пример использования inotify в приложении на Python с использованием пакета inotify:

```python
#!/usr/bin/python3

import inotify.adapters

def _main():
    i = inotify.adapters.Inotify()
    i.add_watch('/mnt/secrets-store/db-password')

    for event in i.event_gen(yield_nones=False):
        (_, type_names, path, filename) = event

        if 'IN_MODIFY' in type_names:
            print("file modified")

if __name__ == '__main__':
    _main()
```

Пример использования inotify в приложении на Go, используя пакет inotify:

```python
watcher, err := inotify.NewWatcher()
if err != nil {
    log.Fatal(err)
}
err = watcher.Watch("/mnt/secrets-store/db-password")
if err != nil {
    log.Fatal(err)
}
for {
    select {
    case ev := <-watcher.Event:
        if ev == 'InModify' {
        	log.Println("file modified")}
    case err := <-watcher.Error:
        log.Println("error:", err)
    }
}
```

#### Ограничения при обновлении секретов

Файлы с секретами не будут обновляться, если будет использован `subPath`.

```yaml
   volumeMounts:
   - mountPath: /app/settings.ini
     name: app-config
     subPath: settings.ini
...
 volumes:
 - name: app-config
   csi:
     driver: secrets-store.csi.deckhouse.io
     volumeAttributes:
       secretsStoreImport: "python-backend"
```

## Скачать мультитул d8 для команд Stronghold

### Официальный сайт Deckhouse Platform Certified Security Edition

Перейдите на официальный сайт и воспользуйтесь [инструкцией](/cli/d8/#как-установить-deckhouse-cli)

### Субдомен вашей Deckhouse Platform Certified Security Edition

Для скачивания мультитула:
1. Перейдите на страницу `tools..<cluster_domain>`, где `<cluster_domain>` — DNS-имя в соответствии с шаблоном из параметра [modules.publicDomainTemplate](/reference/api/global.html#parameters-modules-publicdomaintemplate) глобальной конфигурации.
1. Выберите Deckhouse CLI для вашей операционной системы.
1. **Для Linux и MacOS:**
   - Добавьте права на выполнение `d8` через `chmod +x d8`.
   - Переместите исполняемый файл в каталог `/usr/local/bin/`.

   **Для Windows:**
    - Распакуйте архив, переместите файл `d8.exe` в выбранный вами каталог и добавьте этот каталог в переменную $PATH операционной системы.
    - Разблокируйте файл `d8.exe`, например, следующим способом:
       - Щелкните правой кнопкой мыши на файле и выберите *Свойства* в контекстном меню.
       - В окне *Свойства* убедитесь, что находитесь на вкладке *Общие*.
       - Внизу вкладки *Общие* вы можете увидеть раздел *Безопасность* с сообщением о блокировке файла.
       - Установите флажок *Разблокировать* или нажмите кнопку *Разблокировать*, затем нажмите *Применить* и *ОК*, чтобы сохранить изменения.
4. Проверьте, что утилита работает:
    ```
    d8 help
    ```
Готово, вы установили `d8 stronghold`.

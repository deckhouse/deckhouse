# Чтение агрегированных API с областью фильтрации: тестовое расширение и e2e-тест

Минимальный агрегированный API-сервер (`scopetoy.deckhouse.io/v1`, namespaced-ресурс `gizmos`, данные
в памяти) и сквозной тест чтения с областью фильтрации на нём. Устройство механизма описано в
[SCOPE_AGGREGATED_API_RU.md](../SCOPE_AGGREGATED_API_RU.md).

| Путь | Содержимое |
|---|---|
| `internal/toy/`, `cmd/toy-apiserver/` | Тестовый сервер на основе библиотеки получателя [`pkg/api-extension-sdk/scope`](../../../../../pkg/api-extension-sdk/scope/README.md), подключённой через директиву `replace`. |
| `deploy/` | Манифесты: неймспейсы, RBAC, варианты Deployment, Service и `APIService`, тестовые пользователи, сервисный аккаунт для e2e. |
| `test/e2e.sh` | Сквозные проверки, запускаются с рабочей машины на кластер. |

Модульные тесты запускаются локально: `go test -race ./...`.

## Требования

- kube-apiserver с патчем `scope-aggregated-api` (`images/kubernetes/kubernetes/patches/`) и
  `permission-browser-apiserver` той же версии Deckhouse.
- Непустой `requestheader-allowed-names` в `kube-system/extension-apiserver-authentication`: при
  пустом списке библиотека не доверяет никому.
- `d8`, `curl`, `jq` на рабочей машине.

## Развёртывание

Все команды выполняются из этого каталога.

### Сборка в кластере (основной вариант)

Реестр образов не нужен. Исходники этого каталога и `pkg/api-extension-sdk` передаются в ConfigMap, init-контейнер собирает бинарный файл в
образе `golang:1.26` (узлы должны иметь возможность его скачать), основной контейнер его запускает.

```shell
d8 k apply -f deploy/00-namespaces.yaml -f deploy/10-rbac.yaml
tar --exclude=bin -czf /tmp/scope-toy-src.tgz -C ../../../../.. pkg/api-extension-sdk modules/140-user-authz/docs/internal/scope-aggregated-api-e2e
d8 k -n scope-toy create configmap scope-toy-src --from-file=src.tgz=/tmp/scope-toy-src.tgz
d8 k apply -f deploy/incluster/20-deployment-incluster.yaml
d8 k apply -f deploy/30-service-apiservice.yaml -f deploy/40-test-users.yaml
d8 k -n scope-toy rollout status deploy/scope-toy
```

После изменения исходников пересоздайте ConfigMap и выполните
`d8 k -n scope-toy rollout restart deploy/scope-toy`.

### Образ из реестра

```shell
IMAGE=<registry>/scope-toy:dev bash hack/build.sh image   # docker build + push
IMAGE=<registry>/scope-toy:dev envsubst < deploy/20-deployment.yaml | d8 k apply -f -
```

Остальные манифесты применяются так же, как выше. Для приватного реестра добавьте `imagePullSecrets` в
Deployment. Манифест `deploy/hostpath/20-deployment-hostpath.yaml` запускает бинарный файл, собранный
`bash hack/build.sh` и скопированный в `/opt/scope-toy/` на узел. Применяйте его через
`NODE=<узел> BASE_IMAGE=<любой образ, доступный узлу> envsubst`; для него на неймспейс `scope-toy` нужна
метка `security.deckhouse.io/pod-policy=privileged`.

## Запуск e2e-теста

Тест выполняется от временного сервисного аккаунта, которому разрешено имперсонировать только тестовых
пользователей, а не от администратора: проверки пользователя без
прав нельзя запускать от `system:masters`, делегированный авторизатор всегда разрешает эту группу.
Скрипт берёт текущий контекст kubeconfig и поддерживает только клиентские сертификаты и статические
токены (плагины exec и OIDC не поддерживаются).

```shell
d8 k apply -f deploy/50-e2e-runner.yaml

umask 077   # kubeconfig ниже содержит токен
TOKEN=$(d8 k -n scope-toy create token scope-toy-e2e --duration=2h)
d8 k config view --raw --minify --flatten -o json | jq --arg token "$TOKEN" '{
    apiVersion: "v1", kind: "Config",
    clusters: [.clusters[0]],
    users: [{name: "scope-toy-e2e", user: {token: $token}}],
    contexts: [{name: "scope-toy-e2e", context: {cluster: .clusters[0].name, user: "scope-toy-e2e"}}],
    "current-context": "scope-toy-e2e"}' > /tmp/scope-toy-e2e.kubeconfig

KUBECONFIG=/tmp/scope-toy-e2e.kubeconfig bash test/e2e.sh
```

В kubeconfig остаются только адрес сервера и CA текущего кластера. Скрипт не исполняемый, поэтому
запускайте его через `bash`. Переменные окружения: `PROJECT=<имя>` добавляет проверку
`project:<имя>`, `REVOKE_DEADLINE` (секунды, по умолчанию 15) ограничивает время отзыва доступа в
watch-запросе, `SKIP_PORT_FORWARD=1` пропускает проверки поддельных заголовков напрямую через Service.

## Замечания по запуску на реальном стенде

- Тестовые неймспейсы становятся проектами multitenancy-manager. RoleBinding в них может ссылаться
  только на ClusterRole, выданные проекту, поэтому читатели неймспейса используют Role этого
  неймспейса (`deploy/40-test-users.yaml`).
- `permission-browser-apiserver` следит за аннотацией `scope.deckhouse.io/resources` через watch по
  объектам `APIService`, поэтому изменение начинает действовать в течение нескольких секунд.

## Очистка

Сначала удалите сервисный аккаунт для e2e, затем всё остальное:

```shell
d8 k delete -f deploy/50-e2e-runner.yaml
rm -f /tmp/scope-toy-e2e.kubeconfig /tmp/scope-toy-src.tgz
d8 k delete -f deploy/30-service-apiservice.yaml -f deploy/40-test-users.yaml
d8 k delete -f deploy/10-rbac.yaml -f deploy/00-namespaces.yaml
```

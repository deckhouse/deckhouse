---
title: "Cloud provider — DVP: подготовка окружения"
description: "Настройка окружения Deckhouse для работы облачного провайдера DVP"
---

Для взаимодействия с ресурсами в DVP компоненты Deckhouse Kubernetes Platform используют API DVP. Для настройки подключения создайте пользователя (ServiceAccount), назначьте ему соответствующие права доступа и сгенерируйте kubeconfig.

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

{% alert level="warning" %}
Если модуль `update-hostname` пакета `cloud-init` не отключён, рекомендуется изменить частоту его запуска с `always` на `once-per-instance`.

Для этого измените конфигурацию модуля `update-hostname` в файле `/etc/cloud/cloud.cfg`:

```yaml
cloud_init_modules:
  ...
  - [update-hostname, once-per-instance]
  ...
```

Также модуль `update-hostname` можно полностью отключить, удалив его из списка модулей `cloud_init_modules` в файле `/etc/cloud/cloud.cfg`.

{% endalert %}

## Создание пользователя

Создайте нового пользователя в кластере DVP с помощью следующей команды:

```bash
d8 k create -f -<<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa-demo
  namespace: default
---
apiVersion: v1
kind: Secret
metadata:
  name: sa-demo-token
  namespace: default
  annotations:
    kubernetes.io/service-account.name: sa-demo
type: kubernetes.io/service-account-token
EOF
```

## Добавление роли

Добавьте роль созданному пользователю в кластере DVP с помощью следующей команды:

```bash
d8 k create -f -<<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: sa-demo-rb
  namespace: default
subjects:
  - kind: ServiceAccount
    name: sa-demo
    namespace: default
roleRef:
  kind: ClusterRole
  name: d8:use:role:manager
  apiGroup: rbac.authorization.k8s.io
EOF
```

## Генерация kubeconfig

Сгенерируйте kubeconfig, который будет использоваться в файле первичной конфигурации кластера:

```bash
cat <<EOF > kubeconfig
apiVersion: v1
clusters:
- cluster:
    server: https://<KUBE-APISERVER-URL>   # Замените на реальный адрес API-сервера кластера.
  name: <CLUSTER-NAME>                     # Замените на имя кластера.
contexts:
- context:
    cluster: <CLUSTER-NAME>                # Замените на имя кластера.
    user: sa-demo
    namespace: default
  name: sa-demo-context
current-context: sa-demo-context
kind: Config
preferences: {}
users:
- name: sa-demo
  user:
    token: $(d8 k get secret sa-demo-token -n default -o json | jq -rc .data.token | base64 -d)
EOF
```

Закодируйте сгенерированный kubeconfig в кодировке Base64 (укажите его в секрете `d8-credentials` в поле `stringData.secret` файла первичной конфигурации):

```bash
base64 kubeconfig | tr -d '\n'
```

## Секрет с учётными данными

Учётные данные для доступа к API родительского кластера хранятся не в ModuleConfig, а в отдельном секрете. Провайдер читает его при запуске компонентов, которые обращаются к API DVP.

Секрет создаётся при установке кластера вместе с остальными ресурсами первичной конфигурации. Если модуль подключается к уже работающему кластеру, секрет применяется после того, как модуль создаст неймспейс. Секрет должен отвечать следующим требованиям:

- имя `d8-credentials`, неймспейс `d8-cloud-provider-dvp`;
- тип `cloud-provider.deckhouse.io/credentials`;
- поле `authScheme` со значением `kubeconfig`;
- поле `secret` с kubeconfig в кодировке Base64.

Поле `identity` для схемы `kubeconfig` не используется.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: kubeconfig
  secret: <KUBE_CONFIG_BASE64>
```

Замените `<KUBE_CONFIG_BASE64>` на kubeconfig в кодировке Base64.

Секрет проверяется вебхуком. Платформа отклоняет секрет, если в поле `authScheme` указана другая схема, поле `secret` пустое, задано поле `identity` или содержимое поля `secret` не декодируется как корректный kubeconfig. Обновление, которое меняет тип секрета, платформа тоже отклоняет.

Чтобы сменить учётные данные, обновите поле `secret`:

```shell
d8 k -n d8-cloud-provider-dvp edit secret d8-credentials
```

В кластерах, переведённых на ModuleConfig, kubeconfig переносится в этот секрет из параметра `provider.kubeconfigDataBase64` ресурса DVPClusterConfiguration.

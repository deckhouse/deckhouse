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

Закодируйте сгенерированный kubeconfig в кодировке Base64 (он указывается в файле первичной конфигурации в таком виде):

```bash
base64 kubeconfig | tr -d '\n'
```

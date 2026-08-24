---
title: Мониторинг SLA кластера
permalink: ru/virtualization-platform/documentation/admin/platform-management/monitoring/sla.html
lang: ru
---

DVP может собирать статистику о доступности компонентов кластера и компонентов самого Deckhouse. Эти данные позволяют оценить степень выполнения SLA, а также получить информацию о доступности в веб-интерфейсе.

Кроме того, с помощью кастомного ресурса [UpmeterRemoteWrite](/modules/upmeter/cr.html#upmeterremotewrite) можно экспортировать метрики доступности по протоколу Prometheus Remote Write.

Чтобы начать собирать метрики доступности и активировать [интерфейс](#интерфейс), включите модуль `upmeter` [в веб-интерфейсе Deckhouse](/modules/console/stable/) или воспользуйтесь командой:

```shell
d8 system module enable upmeter
```

## Настройка модуля

Модуль `upmeter` настраивается с помощью ModuleConfig `upmeter`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: upmeter
spec:
  version: 3
  enabled: true
  settings:
```

Перечень всех настроек доступен [в документации модуля](/modules/upmeter/configuration.html).

## Интерфейс

DVP предоставляет два веб-интерфейса для оценки доступности:

1. Страница статуса.

   Получить адрес страницы можно в веб-интерфейсе на главной странице в разделе «Инструменты» (блок «Статус-страница»), или выполнив команду:
   
   ```shell
   d8 k -n d8-upmeter get ing status -o jsonpath='{.spec.rules[*].host}'
   ``` 

   Пример веб-интерфейса страницы статуса:
   
   ![Пример веб-интерфейса страницы статуса](/images/upmeter/status.png)

1. Страница доступности компонентов.

   Получить адрес страницы можно в веб-интерфейсе на главной странице в разделе «Инструменты» (блок «Доступность компонентов»), или выполнив команду:
   
   ```shell
   d8 k -n d8-upmeter get ing webui -o jsonpath='{.spec.rules[*].host}'
   ``` 

   Пример страницы доступности компонентов:
   
   ![Пример графиков по метрикам из upmeter в Grafana](/images/upmeter/image1.png)

## Экспорт метрик статуса
 
Пример конфигурации UpmeterRemoteWrite для экспорта метрик статуса по протоколу [Prometheus Remote Write](https://docs.sysdig.com/en/docs/installation/prometheus-remote-write/):

```yaml
apiVersion: deckhouse.io/v1
kind: UpmeterRemoteWrite
metadata:
  labels:
    heritage: upmeter
    module: upmeter
  name: victoriametrics
spec:
  additionalLabels:
    cluster: cluster-name
    some: fun
  config:
    url: https://upmeter-victoriametrics.whatever/api/v1/write
    basicAuth:
      password: "Cdp#Cd.OxfZsx4*89SZ"
      username: upmeter
  intervalSeconds: 300
```

## Аутентификация

По умолчанию для аутентификации используется модуль [`user-authn`](/modules/user-authn/). Также можно настроить аутентификацию через [`externalAuthentication`](/modules/upmeter/configuration.html#parameters-auth-externalauthentication).
Если эти варианты отключены, модуль включит базовую аутентификацию со сгенерированным паролем.

Посмотреть сгенерированный пароль можно с помощью команды:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.webui.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
d8 k -n d8-upmeter delete secret/basic-auth-webui
```

Посмотреть сгенерированный пароль для страницы статуса можно командой:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.status.password'
```

Чтобы сгенерировать новый пароль для страницы статуса, нужно удалить секрет:

```shell
d8 k -n d8-upmeter delete secret/basic-auth-status
```

> **Внимание.** Параметры `auth.status.password` и `auth.webui.password` больше не поддерживаются.

## Особенности работы подов upmeter

Тесты upmeter создают временные поды для проверки работоспособности компонентов Kubernetes. Поэтому некоторые поды периодически удаляются, остаются в состоянии `Pending` или перемещаются между узлами.

Ниже описано, какие объекты участвуют в проверках:

- `upmeter-probe-scheduler` — проверка планировщика. Тест создает под, размещает его на узле и затем удаляет.
- `upmeter-probe-controller-manager` — проверка `kube-controller-manager`. Тест создает StatefulSet и убеждается, что StatefulSet создал под. Размещение пода на узле в этом тесте не проверяется, поэтому создается под, который заведомо не может быть размещен и остаётся в состоянии `Pending`. Затем StatefulSet удаляется, и проверяется, что созданный им под тоже удален.
- `smoke-mini` — проверка сетевой связности между узлами. Создаются пять StatefulSet с одной репликой каждый. Тест проверяет связность между подами `smoke-mini` и с подами `upmeter-agent` на master-узлах. Раз в минуту один из подов `smoke-mini` переносится на другой узел.

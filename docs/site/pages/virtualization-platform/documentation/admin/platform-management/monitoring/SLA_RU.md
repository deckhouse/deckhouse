---
title: Мониторинг SLA кластера
permalink: ru/virtualization-platform/documentation/admin/platform-management/monitoring/sla.html
lang: ru
---

DVP может собирать статистику о доступности компонентов кластера и компонентов самого Deckhouse. Эти данные позволяют оценить степень выполнения SLA, а также получить информацию о доступности в веб-интерфейсе.

Кроме того, с помощью кастомного ресурса [UpmeterRemoteWrite](/modules/upmeter/cr.html#upmeterremotewrite) можно экспортировать метрики доступности по протоколу Prometheus Remote Write.

Чтобы начать собирать метрики доступности и активировать [интерфейс](#интерфейс), включите модуль `upmeter` [в веб-интерфейсе Deckhouse](/modules/console/stable/) или с помощью следующей команды:

```shell
d8 platform module enable upmeter
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
- Страница статуса.

  Получить адрес страницы можно в веб-интерфейсе на главной странице в разделе «Инструменты» (плитка «Статус-страница»), или выполнив команду:
  
  ```shell
  d8 k -n d8-upmeter get ing status -o jsonpath='{.spec.rules[*].host}'
  ``` 

  Пример веб-интерфейса страницы статуса:
  
  ![Пример веб-интерфейса страницы статуса](/images/upmeter/status.png)

- Страница доступности компонентов.

  Получить адрес страницы можно в веб-интерфейсе на главной странице в разделе «Инструменты» (плитка «Доступность компонентов»), или выполнив команду:
  
  ```shell
  d8 k -n d8-upmeter get ing upmeter -o jsonpath='{.spec.rules[*].host}'
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

По умолчанию для аутентификации используется модуль [`user-authn`](/modules/user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит базовую аутентификацию со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

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

> **Внимание!** Параметры `auth.status.password` и `auth.webui.password` больше не поддерживаются.

## FAQ

### Почему некоторые поды upmeter периодически удаляются или не могут разместиться?

В модуле реализованы тесты доступности и оценки работоспособности различных контроллеров Kubernetes. Тесты выполняется путем создания и удаления временных подов.

Объекты `upmeter-probe-scheduler`, отвечают за проверку работоспособности планировщика. В рамках теста создается под, который размещается на узел. Затем этот под удаляется.

Объекты `upmeter-probe-controller-manager` отвечают за тестирование работоспособности `kube-controller-manager`.  
В рамках теста создается StatefulSet, и проверяется что данный объект породил под (т.к. фактическое размещение пода не требуется и проверяется в рамках другого теста, то создается под который гарантированно не может разместиться, т.е. находится в состоянии `Pending`). Затем StatefulSet удаляется и выполняется проверка, что порожденный им под удалился.

Объекты `smoke-mini` реализуют тестирование сетевой связности между узлами.
Для проверки размещаются пять StatefulSet с одной репликой. В рамках теста проверяется связность как между подами `smoke-mini`, так и сетевая связность с подами `upmeter-agent`, работающими на master-узлах.  
Раз в минуту один из подов `smoke-mini` переносится на другой узел.

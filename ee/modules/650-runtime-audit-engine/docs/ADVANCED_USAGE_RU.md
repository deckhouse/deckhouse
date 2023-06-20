---
title: "Модуль runtime-audit-engine: расширенная конфигурация"
---

## Включение логов для отладки

### Falco

Получите текущий уровень логирования (используется утилита [yq](https://github.com/mikefarah/yq)):

```shell
kubectl -n d8-runtime-audit-engine get configmap runtime-audit-engine -o yaml | yq e '.data."falco.yaml"' - | yq e .log_level - 
```

Выполните следующие шаги, чтобы установить уровень логирования `debug`:
- С помощью следующей команды отредактируйте конфигурацию:

  ```shell
  kubectl -n d8-runtime-audit-engine edit configmap runtime-audit-engine
  ```

- Найдите или добавьте параметр `log_level`, установив его в `debug`.

  Пример:

  ```yaml
  log_level: debug
  ```

### Falcosidekick

Если переменная окружения `DEBUG` установлена в `true`, то в stdout будет выводиться также содержимое всех запросов, с помощью которых falcosidekick передает данные во внешние системы.

Получите значение переменной окружения `DEBUG`:

```shell
kubectl -n d8-runtime-audit-engine get daemonset runtime-audit-engine -o yaml | 
  yq e '.spec.template.spec.containers[] | select(.name == "falcosidekick") | .env[] | select(.name == "DEBUG") | .value' -
```

Выполните следующие шаги, чтобы включить режим отладки.

- С помощью следующей команды отредактируйте конфигурацию DaemonSet `runtime-audit-engine`:

  ```shell
  kubectl -n d8-runtime-audit-engine edit daemonset runtime-audit-engine
  ```

- Добавьте (или исправьте) параметр `env` для контейнера `falcosidekick`, установив переменную окружения `DEBUG` в `"true"`.

  Пример:

  ```yaml
  env:
    - name: DEBUG
      value: "true"
  ```

## Просмотр метрик

Для получения метрик можно использовать PromQL-запрос `falco_events{}`:

```shell
kubectl -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
  curl -s http://127.0.0.1:9090/api/v1/query\?query\=falco_events | jq
```

## Эмуляция события Falco

Есть два способа созданиясобытия Falco:

- Использование CLI-утилиты [event-generator](https://github.com/falcosecurity/event-generator).

  Используйте следующую команду для запуска Пода и генерации всех событий:

  ```shell
  kubectl run falco-event-generator --image=falcosecurity/event-generator run
  ```

- Использование HTTP-эндпоинта `/test` [Falcosidekick](https://github.com/falcosecurity/falcosidekick), чтобы послать событие на все подключенные _выходы_ (outputs).

  - Получите список Подов в пространстве имен `d8-runtime-audit-engine`:
  
    ```shell
    kubectl -n d8-runtime-audit-engine get pods
    ```
  
    Пример вывода:

    ```text
    NAME                         READY   STATUS    RESTARTS   AGE
    runtime-audit-engine-4cpjc   4/4     Running   0          3d12h
    runtime-audit-engine-rn7nj   4/4     Running   0          3d12h
    ```

  - Настройте проброс портов из Пода (в примере `runtime-audit-engine-4cpjc`) на localhost:

    ```shell
    kubectl -n d8-runtime-audit-engine port-forward runtime-audit-engine-4cpjc 2801:2801
    ```

  - Создайте нужное событие, выполнив запрос:

    ```shell
    curl -X POST -H "Content-Type: application/json" -H "Accept: application/json" localhost:2801/test
    ```
  
  - Проверьте метрики:
  
    ```shell
    kubectl -n d8-monitoring exec -it prometheus-main-0 prometheus --  \
      curl -s http://127.0.0.1:9090/api/v1/query\?query\=falco_events | jq
    ```

    Пример части вывода:
  
    ```json
    {
      "metric": {
        "__name__": "falco_events",
        "container": "kube-rbac-proxy",
        "instance": "192.168.199.60:8766",
        "job": "runtime-audit-engine",
        "node": "dev-master-0",
        "priority": "Debug",
        "rule": "Test rule",
        "tier": "cluster"
      },
      "value": [
        1687150913.828,
        "2"
      ]
    }
    ```

---
title: "Модуль runtime-audit-engine: примеры конфигурации"
---

## Как собирать события?

Pod'ы `runtime-audit-engine` выводят все события в стандартный вывод.
После эти события могут быть собраны [агентами log-shipper](../460-log-shipper/) и отправлены в хранилище логов.

Пример [ClusterLoggingConfig](/460-log-shipper/cr.html#clusterloggingconfig) для `log-shipper`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: falco-events
spec:
  destinationRefs:
  - xxxx
  kubernetesPods:
    namespaceSelector:
      matchNames:
      - d8-runtime-audit-events
  labelsFilter:
  - operator: Regex
    values: ["\{.*"] # to collect only JSON logs
    field: "message"
  type: KubernetesPods
```

## Как оповещать о критических событиях?

Метрики о событиях автоматически собираются в Prometheus. 
Добавьте [CustomPrometheusRule](../300-prometheus/cr.html#customprometheusrules) в кластер, чтобы включить оповещения.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: falco-critical-alerts
spec:
  groups:
  - name: falco-critical-alerts
    rules:
    - alert: FalcoCriticalAlertsAreFiring
      for: 1m
      annotations:
        description: |
          There is a suspicious activity on a node {{ $labels.node }}. 
          Check you events journal for more details.
        summary: Falco detects a critical security incident
      expr: |
        sum by (node) (rate(falco_events{priority="Critical"}[5m]) > 0)
```

> NOTE: Оповещения лучше всего работаю в комбинации с хранилищами событий, такими как Elasticsearch или Loki. Оповещения подсказывают о подозрительном поведении на узле.
> Следующий шаг после получения оповещения - пойти в хранилище и посмотреть на сами события.

## Как применить правила для Falco, которые я нашел в интернете?

Структура правил Falco отличается от схемы CRD.
Это связано с ограничением возможности проверки правильности ресурсов в Kubernetes.

Чтобы упростить процесс миграции для применения правил Falco в Deckhouse
был добавлен скрипт для конвертации правил Falco в ресурсы [FalcoAuditRules](cr.html#falcoauditrules).

```shell
git clone github.com/deckhouse/deckhouse
cd deckhouse/ee/modules/650-runtime-audit-engine/hack/fav-converter
go run main.go -input /path/to/falco/rule_example.yaml > ./my-rules-cr.yaml
```

Пример результата работы скрипта:

```yaml
# /path/to/falco/rule_example.yaml
- rule: Linux Cgroup Container Escape Vulnerability (CVE-2022-4092)
  desc: "This rule detects an attempt to exploit a container escape vulnerability in the Linux Kernel."
  condition: container.id != "" and proc.name = "unshare" and spawned_process and evt.args contains "mount" and evt.args contains "-o rdma" and evt.args contains "/release_agent"
  output: "Detect Linux Cgroup Container Escape Vulnerability (CVE-2022-4092) (user=%user.loginname uid=%user.loginuid command=%proc.cmdline args=%proc.args)"
  priority: CRITICAL
  tags: [process, mitre_privilege_escalation]
```

```yaml
# ./my-rules-cr.yaml
apiversion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: rule-example
spec:
    rules:
    - rule:
        name: Linux Cgroup Container Escape Vulnerability (CVE-2022-4092)
        condition: container.id != "" and proc.name = "unshare" and spawned_process and evt.args contains "mount" and evt.args contains "-o rdma" and evt.args contains "/release_agent"
        desc: This rule detects an attempt to exploit a container escape vulnerability in the Linux Kernel.
        output: Detect Linux Cgroup Container Escape Vulnerability (CVE-2022-4092) (user=%user.loginname uid=%user.loginuid command=%proc.cmdline args=%proc.args)
        priority: Critical
        tags:
        - process
        - mitre_privilege_escalation
```

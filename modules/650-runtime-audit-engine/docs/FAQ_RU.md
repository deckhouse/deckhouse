---
title: "FAQ"
---



## Как собирать события?

Поды `runtime-audit-engine` выводят все события в стандартный вывод.
Далее агенты log-shipper могут собирать их и отправлять в хранилище логов.

Пример конфигурации [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) для модуля `log-shipper`:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingConfig
metadata:
  name: falco-events
spec:
  destinationRefs:
  - xxxx
  kubernetesPods:
    namespaceSelector:
      labelSelector:
        matchExpressions:
        - key: "kubernetes.io/metadata.name"
          operator: In
          values: [d8-runtime-audit-engine]
  labelFilter:
  - operator: Regex
    values: ["\\{.*"] # to collect only JSON logs
    field: "message"
  type: KubernetesPods
```

## Как оповещать о критических событиях?

Prometheus автоматически собирает метрики о событиях.
Чтобы включить оповещения, добавьте в кластер правило [CustomPrometheusRule](/modules/prometheus/cr.html#customprometheusrules).

Пример настройки такого правила:

{% raw %}

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
        sum by (node) (rate(falcosecurity_falcosidekick_falco_events_total{priority="Critical"}[5m]) > 0)
```

{% endraw %}

{{< alert >}}
Алерты лучше всего работают в комбинации с хранилищами событий, такими как Elasticsearch или Loki. Их задача — оповестить пользователя о подозрительном поведении на узле.
После получения алерта рекомендуется «пойти» в хранилище и посмотреть на события, которые его вызвали.
{{< /alert >}}


## Как применить правила для Falco, найденные в интернете?

Структура правил Falco отличается от схемы CRD.
Это связано со сложностями при проверке правильности ресурсов в Kubernetes.

Скрипт для конвертации правил Falco в ресурсы [FalcoAuditRules](cr.html#falcoauditrules) встроен в функционал утилиты `d8`.  
С его помощью можно применять правила Falco в Deckhouse:

```shell
d8 tools far-converter /path/to/falco/rule_example.yaml > ./my-rules-cr.yaml
```

Пример результата работы скрипта:

```yaml
# /path/to/falco/rule_example.yaml
- macro: spawned_process
  condition: (evt.type in (execve, execveat) and evt.dir=<)

- rule: Linux Cgroup Container Escape Vulnerability (CVE-2022-0492)
  desc: "This rule detects an attempt to exploit a container escape vulnerability in the Linux Kernel."
  condition: container.id != "" and proc.name = "unshare" and spawned_process and evt.args contains "mount" and evt.args contains "-o rdma" and evt.args contains "/release_agent"
  output: "Detect Linux Cgroup Container Escape Vulnerability (CVE-2022-0492) (user=%user.loginname uid=%user.loginuid command=%proc.cmdline args=%proc.args)"
  priority: CRITICAL
  tags: [process, mitre_privilege_escalation]
```

```yaml
# ./my-rules-cr.yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: rule-example
spec:
  rules:
    - macro:
        name: spawned_process
        condition: (evt.type in (execve, execveat) and evt.dir=<)
    - rule:
        name: Linux Cgroup Container Escape Vulnerability (CVE-2022-0492)
        condition: container.id != "" and proc.name = "unshare" and spawned_process and evt.args contains "mount" and evt.args contains "-o rdma" and evt.args contains "/release_agent"
        desc: This rule detects an attempt to exploit a container escape vulnerability in the Linux Kernel.
        output: Detect Linux Cgroup Container Escape Vulnerability (CVE-2022-0492) (user=%user.loginname uid=%user.loginuid command=%proc.cmdline args=%proc.args)
        priority: Critical
        tags:
          - process
          - mitre_privilege_escalation
```

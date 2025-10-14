---
title: Аудит событий безопасности
permalink: security/events/runtime-audit.html
description: "Настройка аудита событий безопасности в Deckhouse Platform Certified Security Edition. Мониторинг runtime безопасности, обнаружение угроз и аудит логирование для анализа безопасности кластера."
lang: ru
---

Deckhouse Platform Certified Security Edition предоставляет встроенные средства поиска угроз безопасности
за счёт анализа событий ядра Linux и аудита событий Kubernetes API.
Deckhouse Platform Certified Security Edition позволяет:

- находить угрозы в окружениях, анализируя приложения и контейнеры;
- обнаруживать попытки применения уязвимостей из базы CVE и признаки запуска криптовалютных майнеров;
- выявлять угрозы Kubernetes, включая:
  - оболочки командной строки, запущенные в контейнерах или подах;
  - контейнеры, работающие в привилегированном режиме;
  - монтирование небезопасных путей в контейнеры (например, `/proc`);
  - попытки чтения секретных данных (например, из `/etc/shadow`).

## Источники данных для аудита безопасности

Deckhouse Platform Certified Security Edition использует два основных источника событий:

- события ядра Linux — с помощью eBPF-драйвера для [системы обнаружения угроз Falco](https://falco.org/);
- события [аудита API Kubernetes](./kubernetes-api-audit.html) — через интеграцию с механизмом Kubernetes auditing и вебхук-интерфейс.

## Минимальные требования

Для получения событий ядра требуется:

- ядро Linux версии 5.8 или выше;
- поддержка [eBPF](https://www.kernel.org/doc/html/v5.8/bpf/btf.html).
  Проверить наличие поддержки можно одним из следующих способов:
  - убедитесь в наличии файла `/sys/kernel/btf/vmlinux`:

    ```shell
    ls -lah /sys/kernel/btf/vmlinux
    ```
  
  - убедитесь, что включён параметр `CONFIG_DEBUG_INFO_BTF`:
  
    ```shell
    grep -E "CONFIG_DEBUG_INFO_BTF=(y|m)" /boot/config-*
    ```

На каждом узле кластера запускаются агенты Falco,
которые потребляют ресурсы в зависимости от количества применяемых правил или собираемых событий.

{% alert level="info" %}
На некоторых системах могут не работать пробы eBPF.
{% endalert %}

## Как включить аудит событий безопасности

1. Убедитесь, что узлы соответствуют [минимальным требованиям](#минимальные-требования).
1. Включите аудит в Deckhouse, используя следующую конфигурацию:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: runtime-audit-engine
   spec:
     enabled: true
   ```

1. (**Опционально**) Если control plane в кластере не управляется Deckhouse Platform Certified Security Edition при помощи [`control-plane-manager`](/modules/control-plane-manager/),
   настройте вебхук аудита API Kubernetes вручную.

Все доступные параметры аудита безопасности доступны [в разделе документации модуля `runtime-audit-engine`](/modules/runtime-audit-engine/configuration.html).

### Настройка вебхука API Kubernetes вручную

{% alert level="info" %}
Настройка вебхука не требуется, если включён модуль [`control-plane-manager`](/modules/control-plane-manager/).
В этом случае при включении модуля [`runtime-audit-engine`](/modules/runtime-audit-engine/)
настройки сбора событий аудита API Kubernetes применятся автоматически.
{% endalert %}

Чтобы настроить вебхук на получение событий аудита от `kube-apiserver`, выполните следующие шаги:

1. Создайте файл `kubeconfig` для вебхука с адресом `https://127.0.0.1:9765/k8s-audit`
   и данными сертификата (`ca.crt`) из Secret `d8-runtime-audit-engine/runtime-audit-engine-webhook-tls`.

   Пример:

   ```yaml
   apiVersion: v1
   kind: Config
   clusters:
   - name: webhook
     cluster:
       certificate-authority-data: BASE64_CA
       server: "https://127.0.0.1:9765/k8s-audit"
   users:
   - name: webhook
   contexts:
   - context:
      cluster: webhook
      user: webhook
     name: webhook
   current-context: webhook
   ```

1. Укажите путь к созданному файлу конфигурации с помощью флага `--audit-webhook-config-file` в манифесте `kube-apiserver`.
1. (**Опционально**) Чтобы собирать события аудита API Kubernetes не только из системных,
   но и пользовательских пространств имён,
   настройте [политики аудита](./kubernetes-api-audit.html#настройка-собственной-политики-аудита).

## Работа с правилами аудита

Для анализа событий безопасности используются правила, определяющие критерии подозрительного поведения.
В Deckhouse Platform Certified Security Edition предусмотрены:

- **встроенные правила**, включая:
  - правила для аудита Kubernetes (располагаются в контейнере `falco` по пути `/etc/falco/k8s_audit_rules.yaml`);
  - правила, удовлетворяющие требованиям приказа ФСТЭК России №118 от 4 июля 2022 г.
    «Требования по безопасности информации к средствам контейнеризации»
    (`fstec`, в формате [кастомного ресурса FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules));

  Чтобы настроить список встроенных правил,
  используйте [параметр `settings.builtInRulesList`](/modules/runtime-audit-engine/configuration.html#parameters-builtinruleslist) модуля [`runtime-audit-engine`](/modules/runtime-audit-engine/).

- **пользовательские правила**, которые задаются через [кастомный ресурс FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules).


### Добавление пользовательского правила

Чтобы добавить правило, создайте [ресурс FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) с необходимыми условиями.
Используйте [синтаксис условий Falco](https://falco.org/docs/concepts/rules/conditions/).
Агенты Falco автоматически применят созданное правило.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: ownership-permissions
spec:
  rules:
  - macro:
      name: spawned_process
      condition: (evt.type in (execve, execveat) and evt.dir=<)
  - rule:
      name: Detect Ownership Change
      desc: detect file permission/ownership change
      condition: >
        spawned_process and proc.name in (chmod, chown) and proc.args contains "/tmp/"
      output: >
        The file or directory below has had its permissions or ownership changed (user=%user.name
        command=%proc.cmdline file=%fd.name parent=%proc.pname pcmdline=%proc.pcmdline gparent=%proc.aname[2])
      priority: Warning
      tags: [filesystem]
```

Дополнительные примеры правил можно найти на следующих ресурсах:

- официальный репозиторий правил Falco;
- [раздел с правилами Falco на Artifact Hub](https://artifacthub.io/packages/search?kind=1&sort=relevance&page=1).

### Применение стороннего правила

Поскольку структура правил Falco отличается от схемы кастомных ресурсов Deckhouse Platform Certified Security Edition,
сторонние правила из интернета необходимо сконвертировать [в ресурс FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) перед применением.

Используйте следующий скрипт для конвертации:

```shell
git clone github.com/deckhouse/deckhouse
cd deckhouse/ee/modules/runtime-audit-engine/hack/far-converter
go run main.go -input /path/to/falco/rule_example.yaml > ./my-rules-cr.yaml
```

Пример результата работы скрипта:

- Изначальное правило:

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

- Ресурс с правилом после конвертации:

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

## Сбор логов и оповещения

Deckhouse Platform Certified Security Edition экспортирует события аудита безопасности в формате метрик Prometheus,
по которым можно настроить оповещения через [ресурс CustomPrometheusRules](/modules/prometheus/cr.html#customprometheusrules).
Это позволяет:

- подключить внешнее хранилище для сбора логов (например, Loki или Elasticsearch);
- настроить оповещения о критических событиях.

### Настройка сбора логов и событий

Все события аудита безопасности выводятся в stdout.
Для сбора и отправки событий в хранилище логов создайте [ресурс ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig), следуя примеру:

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
      labelSelector:
        matchExpressions:
        - key: "kubernetes.io/metadata.name"
          operator: In
          values: [d8-runtime-audit-engine]
  labelFilter:
  - operator: Regex
    values: ["\\{.*"] # Для сбора логов только в формате JSON.
    field: "message"
  type: KubernetesPods
```

### Настройка оповещений о критических событиях

Для создания оповещений о критических событиях создайте [объект CustomPrometheusRules](/modules/prometheus/cr.html#customprometheusrules), следуя примеру:
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
        sum by (node) (rate(falco_events{priority="Critical"}[5m]) > 0)
```

{% endraw %}

### Просмотр метрик

Для получения Prometheus-метрик используйте PromQL-запрос `falcosecurity_falcosidekick_falco_events_total{}`:

```shell
d8 k -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
  curl -s "http://127.0.0.1:9090/api/v1/query?query=falcosecurity_falcosidekick_falco_events_total" | jq
```

## Отладка и эмуляция событий

Для отладки и эмуляции событий безопасности в Deckhouse Platform Certified Security Edition можно использовать:

- утилиту `event-generator`;
- HTTP-эндпоинт `/test` сервиса `falcosidekick`.

### Включение логов для отладки

В Falco по умолчанию используется отладочный уровень логирования `debug`.

В Falcosidekick по умолчанию отладочное логирование отключено.
Для включения установите [параметр `spec.settings.debugLogging`](/modules/runtime-audit-engine/configuration.html#parameters-debuglogging) в `true`, следуя примеру

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: runtime-audit-engine
spec:
  enabled: true
  settings:
    debugLogging: true
```

### Настройка эмуляции событий

#### Falco

Утилита `event-generator` позволяет генерировать
различные подозрительные действия (например, системные вызовы или события аудита API Kubernetes).

Используйте следующую команду для запуска тестового набора событий в кластере Kubernetes:

```shell
d8 k run falco-event-generator --image=falcosecurity/event-generator run
```

Если вам нужно сымитировать определённое действие,
обратитесь к руководству утилиты.

#### Falcosidekick

Чтобы имитировать отправку тестовых событий для сервиса `falcosidekick`,
используйте HTTP-эндпоинт `/test`:

1. Создайте тестовое событие, выполнив следующую команду:

   ```shell
   nsenter -t $(pidof falcosidekick) curl -X POST -H "Content-Type: application/json" -H "Accept: application/json" http://localhost:2801/test
   ```

1. Проверьте метрику события:

   ```shell
   d8 k -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
     curl -s "http://127.0.0.1:9090/api/v1/query?query=falcosecurity_falcosidekick_falco_events_total" \
     | jq '.data.result.[] | select (.metric.priority_raw == "debug")'
   ```

   Пример вывода:

   ```console
   {
     "metric": {
       "__name__": "falcosecurity_falcosidekick_falco_events_total",
       "container": "kube-rbac-proxy",
       "hostname": "falcosidekick",
       "instance": "192.168.208.7:4212",
       "job": "runtime-audit-engine",
       "node": "dev-master-0",
       "priority": "1",
       "priority_raw": "debug",
       "rule": "Test rule",
       "source": "internal",
       "tier": "cluster"
     },
     "value": [
       1744234729.799,
       "1"
     ]
   }
   ```

---
title: "Модуль runtime-audit-engine"
description: Описание модуля runtime-audit-engine Deckhouse, предназначенного для поиска угроз безопасности в кластере Kubernetes.
---

## Описание

Модуль предназначен для поиска угроз безопасности.

Модуль собирает события ядра Linux и итоги аудита API Kubernetes (с помощью плагина `k8saudit`), обогащает их метаданными о подах Kubernetes и генерирует события аудита безопасности по установленным правилам.

Модуль runtime-audit-engine:
* Находит угрозы в окружениях, анализируя приложения и контейнеры.
* Помогает обнаружить попытки применения уязвимостей из базы CVE и запуска криптовалютных майнеров.
* Повышает безопасность Kubernetes, выявляя:
  * оболочки командной строки, запущенные в контейнерах или подах в Kubernetes;
  * контейнеры, работающие в привилегированном режиме; монтирование небезопасных путей (например, `/proc`) в контейнеры;
  * попытки чтения секретных данных из, например, `/etc/shadow`.

## Архитектура

Ядро модуля основано на системе обнаружения угроз [Falco](https://falco.org/).
Deckhouse запускает агенты Falco (объединены в DaemonSet) на каждом узле, после чего те приступают к сбору событий ядра и данных, полученных в ходе аудита Kubernetes.

![Falco DaemonSet](../../images/650-runtime-audit-engine/falco_daemonset.svg)
<!--- Source: https://docs.google.com/drawings/d/1NZ91z8NXNiuS50ybcMoMsZI3SbQASZXJGLANdaNNm_U --->

{% alert %}
Для максимальной безопасности разработчики Falco рекомендуют запускать Falco как systemd-сервис, однако в кластерах Kubernetes с поддержкой автомасштабирования это может быть затруднительно. Дополнительные средства безопасности Deckhouse (реализованные другими модулями), такие как multi-tenancy или политики контроля создаваемых ресурсов, предоставляют достаточный уровень безопасности для предотвращения атак на DaemonSet Falco.
{% endalert %}

Один под Falco состоит из четырех контейнеров:
![Falco Pod](../../images/650-runtime-audit-engine/falco_pod.svg)
<!--- Source: https://docs.google.com/drawings/d/1rxSuJFs0tumfZ56WbAJ36crtPoy_NiPBHE6Hq5lejuI --->

1. `falco` — собирает события, обогащает их метаданными и отправляет их в stdout.
2. `rules-loader` — собирает custom resourcе'ы ([FalcoAuditRules](cr.html#falcoauditrules)) из Kubernetes и сохраняет их в общую папку.
3. `falcosidekick` — принимает события от `Falco` и перенаправляет их разными способами. По умолчанию экспортирует события как метрики, по которым потом можно настроить алерты. [Исходный код Falcosidekick](https://github.com/falcosecurity/falcosidekick).
4. `kube-rbac-proxy` — защищает endpoint метрик `falcosidekick` (запрещает неавторизованный доступ).

## Правила аудита

Сборка событий сама по себе не дает ничего, поскольку объем данных, собираемый с ядра Linux, слишком велик для анализа человеком.
Правила позволяют решить эту проблему: события отбираются по определенным условиям. Условия настраиваются на выявление любой подозрительной активности.

В основе каждого правила лежит выражение, содержащее определенное условие, написанное в соответствии [с синтаксисом условий](https://falco.org/docs/rules/conditions/).

### Встроенные правила

Существуют два встроенных набора правил, которые нельзя отключить.
Они помогают выявить проблемы с безопасностью Deckhouse и с самим модулем `runtime-audit-engine`:

- `/etc/falco/k8s_audit_rules.yaml` — правила для аудита Kubernetes.

### Пользовательские правила

Добавить пользовательские правила можно с помощью custom resource [FalcoAuditRules](cr.html#falcoauditrules).
У каждого агента Falco есть sidecar-контейнер с экземпляром [shell-operator](https://github.com/flant/shell-operator).
Этот экземпляр считывает правила из custom resource'ов Kubernetes, конвертирует их в правила Falco и сохраняет правила Falco в директорию `/etc/falco/rules.d/` пода.
При добавлении нового правила Falco автоматически обновляет конфигурацию.

![Falco shell-operator](../../images/650-runtime-audit-engine/falco_shop.svg)
<!--- Source: https://docs.google.com/drawings/d/13MFYtiwH4Y66SfEPZIcS7S2wAY6vnKcoaztxsmX1hug --->

Такая схема позволяет использовать подход «Инфраструктура как код» при работе с правилами Falco.

## Требования

### Операционная система

Модуль использует драйвер eBPF для Falco при сборке событий ядра операционной системы. Этот драйвер особенно полезен в окружениях, в которых невозможна сборка модуля ядра (например, GKE, EKS и другие решения Managed Kubernetes).
У драйвера eBPF есть следующие требования:
* Ядро Linux >= 5.8.
* Включённый [eBPF](https://www.kernel.org/doc/html/v5.8/bpf/btf.html). Проверьте командой `ls -lah /sys/kernel/btf/vmlinux`, либо найдите `CONFIG_DEBUG_INFO_BTF=y` в списке параметров сборки ядра.

> На некоторых системах пробы (probe) eBPF могут не работать.

### Процессор / Память

Агенты Falco работают на каждом узле. Поды агентов потребляют ресурсы в зависимости от количества применяемых правил или собираемых событий.

## Kubernetes Audit Webhook

Режим [Webhook audit mode](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#webhook-backend) должен быть настроен на получение событий аудита от `kube-apiserver`.
Если модуль [control-plane-manager](../040-control-plane-manager/) включен, настройки автоматически применятся при включении модуля `runtime-audit-engine`.

В кластерах Kubernetes, в которых control plane не управляется Deckhouse, webhook необходимо настроить вручную. Для этого:

1. Создайте файл kubeconfig для webhook с адресом `https://127.0.0.1:9765/k8s-audit` и CA (ca.crt) из Secret'а `d8-runtime-audit-engine/runtime-audit-engine-webhook-tls`.

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

2. Добавьте к `kube-apiserver` флаг `--audit-webhook-config-file`, который будет указывать на файл, созданный на предыдущем шаге.

{% alert level="warning" %}
Не забудьте настроить audit policy, поскольку Deckhouse по умолчанию собирает только события аудита Kubernetes для системных пространств имен.
Пример конфигурации можно найти в документации модуля [control-plane-manager](../040-control-plane-manager/).
{% endalert %}

## Алерты

Если несколько подов `runtime-audit-engine` не назначены на узлы планировщиком, будет сгенерирован алерт `D8RuntimeAuditEngineNotScheduledInCluster`.

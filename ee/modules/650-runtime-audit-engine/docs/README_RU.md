---
title: "Модуль runtime-audit-engine"
---

## Описание

Модуль предназначен для поиска угроз безопасности.
Он собирает события ядра Linux и итоги аудита API Kubernetes, обогащает их метаданными о Pod'ах Kubernetes и генерирует события аудита безопасности по установленным правилам.

Модуль runtime-audit-engine:
* Находит угрозы в окружениях, анализируя приложения и контейнеры.
* Помогает обнаружить попытки применения уязвимостей из базы CVE и запуска криптовалютных майнеров.
* Повышает безопасность Kubernetes, выявляя:
  * Оболочки командной строки, запущенные в контейнерах или Pod'ах в Kubernetes.
  * Контейнеры, работающие в привилегированном режиме; монтирование небезопасных путей (например, `/proc`) в контейнеры.
  * Попытки чтения секретных данных из, например, `/etc/shadow`.

## Архитектура

Ядро модуля основано на системе обнаружения угроз [Falco](https://falco.org/).
Deckhouse запускает агенты Falco (объединены в DaemonSet) на каждом узле, после чего те приступают к сбору событий ядра и данных, полученных в ходе аудита Kubernetes.

![Falco DaemonSet](../../images/650-runtime-audit-engine/falco_daemonset.svg)
<!--- Source: https://docs.google.com/drawings/d/1NZ91z8NXNiuS50ybcMoMsZI3SbQASZXJGLANdaNNm_U --->

> Для максимальной безопасности разработчики Falco рекомендуют запускать Falco как systemd-сервис, однако в кластерах Kubernetes с поддержкой автомасштабирования это может быть затруднительно.
> Дополнительные средства безопасности Deckhouse (реализованные другими модулями), такие как multitenancy или политики контроля создаваемых ресурсов, предоставляют достаточный уровень безопасности для предотвращения атак на DaemonSet Falco.

Один Pod Falco состоит из пяти контейнеров:
![Falco Pod](../../images/650-runtime-audit-engine/falco_pod.svg)
<!--- Source: https://docs.google.com/drawings/d/1rxSuJFs0tumfZ56WbAJ36crtPoy_NiPBHE6Hq5lejuI --->

1. `falco-driver-loader` — контейнер для запуска; собирает eBPF-программу и сохраняет ее в общую папку для дальнейшего использования системой Falco.
2. `falco` — собирает события, обогащает их метаданными и отправляет их в stdout.
3. `rules-loader` — собирает custom resourcе'ы ([FalcoAuditRules](cr.html#falcoauditrules)) из Kubernetes и сохраняет их в общую папку.
4. `falcosidekick` — экспортирует события как метрики, по которым потом можно настроить алерты.
5. `kube-rbac-proxy` — защищает endpoint метрик `falcosidekick` (запрещает неавторизованный доступ).

## Правила аудита

Сборка событий сама по себе не дает ничего, поскольку объем данных, собираемый с ядра Linux, слишком велик для анализа человеком.
Правила позволяют решить эту проблему: события отбираются по определенным условиям. Условия настраиваются на выявление любой подозрительной активности.

В основе каждого правила лежит выражение, содержащее определенное условие, написанное в соответствии с [синтаксисом условий](https://falco.org/docs/rules/conditions/).

### Встроенные правила

Существует два встроенных набора правил, которые нельзя отключить.
Они помогают выявить проблемы с безопасностью Deckhouse и с самим модулем `runtime-audit-engine`:

- `/etc/falco/falco_rules.yaml` — правила для системных вызовов;
- `/etc/falco/k8s_audit_rules.yaml` — правила для аудита Kubernetes.

### Пользовательские правила

Добавить пользовательские правила можно с помощью custom resource [FalcoAuditRules](cr.html#falcoauditrules).
У каждого агента Falco есть sidecar-контейнер с экземпляром [shell-operator](https://github.com/flant/shell-operator).
Этот экземпляр считывает правила из custom resource'ов Kubernetes и сохраняет их в директорию `/etc/falco/rules.d/` Pod'а.
При добавлении нового правила Falco автоматически обновляет конфигурацию.

![Falco shell-operator](../../images/650-runtime-audit-engine/falco_shop.svg)
<!--- Source: https://docs.google.com/drawings/d/13MFYtiwH4Y66SfEPZIcS7S2wAY6vnKcoaztxsmX1hug --->

Такая схема позволяет использовать подход "Инфраструктура как код" при работе с правилами Falco.

## Требования

### Операционная система

Модуль использует драйвер eBPF для Falco при сборке событий ядра операционной системы. Этот драйвер особенно полезен в окружениях, в которых невозможна сборка модуля ядра (например, GKE, EKS и другие решения Managed Kubernetes).
Однако у драйвера eBPF есть и ограничения:
* На некоторых системах probe'ы eBPF могут не работать;
* Минимальная необходимая версия ядра Linux — 4.14; проект Falco рекомендует использовать ядро LTS версии 4.14/4.19 или выше.

### Процессор / Память

Агенты Falco работают на каждом узле. Pod'ы агентов потребляют ресурсы в зависимости от количества применяемых правил или собираемых событий.

## Kubernetes Audit Webhook

Режим [Webhook audit mode](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#webhook-backend) должен быть настроен на получение событий аудита от `kube-apiserver`.
Если модуль [control-plane-manager](../040-control-plane-manager/) включен, настройки автоматически применятся при включении модуля `runtime-audit-engine`.

В кластерах Kubernetes, в которых control plane не управляется Deckhouse, webhook необходимо настроить вручную. Для этого:

1. Создайте файл kubeconfig для webhook с адресом `https://127.0.0.1:9765/k8s-audit` и CA (ca.crt) из секрета `d8-runtime-audit-engine/runtime-audit-engine-webhook-tls`.

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

> **Внимание!** Не забудьте настроить audit policy, поскольку Deckhouse по умолчанию собирает только события аудита Kubernetes для системных пространств имен.
> Пример конфигурации можно найти в документации модуля [control-plane-manager](../040-control-plane-manager/).

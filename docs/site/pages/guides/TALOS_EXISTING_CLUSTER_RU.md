---
title: Установка Deckhouse Platform в существующий Talos-кластер
permalink: ru/guides/talos-existing-cluster.html
description: Руководство по установке Deckhouse Platform в существующий Talos-кластер.
lang: ru
layout: sidebar-guides
relatedLinks:
  - title: "Установка Deckhouse Platform в существующий кластер"
    url: /products/kubernetes-platform/gs/existing/step3.html
  - title: "Настройки модуля deckhouse"
    url: /modules/deckhouse/configuration.html
  - title: "Bundle и управление модулями"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/
  - title: "Изменение Talos MachineConfig с помощью патчей"
    url: "https://docs.siderolabs.com/talos/v1.13/configure-your-talos-cluster/system-configuration/patching"
---

Эта инструкция подходит для ситуации, когда Kubernetes-кластер на базе [Talos Linux](https://www.siderolabs.com/talos-linux) уже создан и работает: control plane запущен, worker-узлы присоединены, CNI установлен, а Kubernetes API доступен через `d8 k`.

Deckhouse Platform (DP) устанавливается поверх готового Kubernetes-кластера в режиме установки в существующий кластер. В примере используется Deckhouse Platform Open, [канал обновлений `EarlyAccess`](/modules/deckhouse/configuration.html#parameters-releasechannel) и [bundle `Managed`](/modules/deckhouse/configuration.html#parameters-bundle).

В этой схеме:

- Talos продолжает управлять ОС, MachineConfig, kubelet, containerd, etcd, control plane, Kubernetes PKI и обновлением Kubernetes;
- существующий CNI продолжает отвечать за сеть подов;
- внешний инфраструктурный провайдер или пользователь продолжает создавать и удалять машины;
- DP устанавливает и обновляет платформенные модули, но не управляет Talos и жизненным циклом узлов.

{% alert level="warning" %}
Значение `bundle` выбирается во время установки и впоследствии не изменяется. Нельзя установить `Managed`, а затем обычным изменением конфигурации переключить его на `Minimal` или `Default`.
{% endalert %}

## Предварительные требования

На компьютере, с которого запускается установка, нужны:

- Docker;
- [Deckhouse CLI](/products/kubernetes-platform/documentation/v1/cli/d8/);
- `yq` для проверки YAML;
- административный kubeconfig Talos-кластера;
- доступ к Kubernetes API;
- HTTPS-доступ к `registry.deckhouse.ru` с компьютера и узлов кластера;
- `talosctl` и talosconfig, если административный kubeconfig ещё не получен.

SSH к Talos-узлам не нужен: установщик работает через Kubernetes API.

Перед установкой рекомендуется сделать снимок etcd средствами Talos и сохранить исходные talosconfig и kubeconfig.

## Настройка рабочих путей

Создайте отдельный каталог для файлов установки и перейдите в него:

```bash
mkdir -p "$PWD/talos-deckhouse-install"
cd "$PWD/talos-deckhouse-install"
```

Все дальнейшие команды предполагают, что текущим остаётся этот каталог. Задайте пути один раз:

```bash
TALOSCONFIG="$PWD/talosconfig"
ADMIN_KUBECONFIG="$PWD/kubeconfig-admin"
INSTALLER_KUBECONFIG="$PWD/kubeconfig-installer"
CONFIG_FILE="$PWD/config.yml"
```

Назначение файлов:

| Переменная | Назначение |
| --- | --- |
| `TALOSCONFIG` | Конфигурация `talosctl` для доступа к Talos API |
| `ADMIN_KUBECONFIG` | Административный kubeconfig для команд `d8 k` на компьютере |
| `INSTALLER_KUBECONFIG` | Переносимая копия административного kubeconfig для Docker-контейнера |
| `CONFIG_FILE` | Конфигурация установки Deckhouse Platform |

Если вы открыли новый терминал, снова перейдите в рабочий каталог и задайте четыре переменные из блока выше.

## Подготовка административного kubeconfig

Дальнейшие действия зависят от того, есть ли у вас административный kubeconfig.

### Если kubeconfig уже есть

Скопируйте его в рабочий каталог:

```bash
cp <ADMIN_KUBECONFIG_PATH> "$ADMIN_KUBECONFIG"
chmod 600 "$ADMIN_KUBECONFIG"
```

{% alert level="info" %}
Имеется в виду kubeconfig для `d8 k`, а не talosconfig для `talosctl`.
{% endalert %}

### Если kubeconfig нужно получить через Talos

Сначала поместите существующий talosconfig в рабочий каталог:

```bash
cp <TALOSCONFIG_PATH> "$TALOSCONFIG"
chmod 600 "$TALOSCONFIG"
```

Укажите адрес control-plane-узла:

```bash
CONTROL_PLANE_ADDRESS=<CONTROL_PLANE_IP_OR_DNS>
```

Получите административный kubeconfig:

```bash
talosctl kubeconfig "$ADMIN_KUBECONFIG" --talosconfig="$TALOSCONFIG" --nodes="$CONTROL_PLANE_ADDRESS" --merge=false
chmod 600 "$ADMIN_KUBECONFIG"
```

По умолчанию `talosctl` возьмёт эндпоинты Talos API из текущего контекста talosconfig. Если нужен другой эндпоинт, добавьте:

```text
--endpoints=<TALOS_API_ENDPOINT>
```

Параметр `--force` используйте только при осознанной перезаписи существующего файла.

### Проверка прав

Проверьте, как Kubernetes определяет пользователя:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" auth whoami
```

Для установки нужен стабильный административный доступ. Административный kubeconfig Talos обычно использует группу `system:masters`. Пользовательский OIDC kubeconfig для установки нежелателен: после включения модуля DP `user-authz` права такого пользователя в системных неймспейсах могут измениться.

Проверьте основные разрешения:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" auth can-i '*' '*' --all-namespaces
d8 k --kubeconfig="$ADMIN_KUBECONFIG" auth can-i create customresourcedefinitions.apiextensions.k8s.io
d8 k --kubeconfig="$ADMIN_KUBECONFIG" auth can-i create clusterroles.rbac.authorization.k8s.io
```

Все три команды должны вывести `yes`.

## Проверка исходного кластера

Проверьте узлы:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get nodes -o wide
```

Все узлы должны быть в состоянии `Ready`.

Проверьте Kubernetes API:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get --raw='/readyz?verbose'
```

В конце ответа должно быть `readyz check passed`.

Проверьте системные поды:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" -n kube-system get pods -o wide
```

До установки DP уже должны работать:

- CNI;
- CoreDNS;
- kube-proxy, если он используется выбранной сетевой схемой;
- компоненты control plane.

Также убедитесь, что версия Kubernetes [поддерживается](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes) выбранной версией DP.

## Проверка границ ответственности

DP не должна одновременно с Talos или внешним инфраструктурным провайдером управлять одними и теми же компонентами.

В таблице приведён список компонентов и их владельцев после установки:

| Компонент | Владелец после установки |
| --- | --- |
| Talos OS и MachineConfig | Talos |
| etcd и Kubernetes control plane | Talos |
| Kubernetes PKI | Talos |
| kubelet и containerd | Talos |
| CNI | Уже установленный внешний CNI |
| CoreDNS и kube-proxy, если используется | Существующий кластер |
| Создание и удаление машин | Внешний инфраструктурный провайдер или пользователь |
| Платформенные модули | DP |

Следующие модули DP должны оставаться выключенными:

- `control-plane-manager`;
- `node-manager`;
- `terraform-manager`;
- `cni-cilium`;
- `kube-dns`;
- `kube-proxy`;
- модули облачных провайдеров (`cloud-provider-*`);
- `registry-packages-proxy`.

{% alert level="warning" %}
Если в Talos-кластере уже установлен Cilium, включать модуль DP `cni-cilium` нельзя: два оператора не должны одновременно управлять одним CNI.
{% endalert %}

Bundle `Managed` включает `ingress-nginx`, `cert-manager`, `local-path-provisioner`, VPA, мониторинг и модуль `user-authz`. До установки проверьте, нет ли в кластере их внешних аналогов:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get storageclass
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get ingressclass
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get deployments -A
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get crd
```

Если компонент уже установлен, заранее определите единственного владельца. Не запускайте одновременно два Ingress-контроллера, два `cert-manager` или два VPA.
Если управление компонентом остаётся за внешним решением, явно отключите соответствующий модуль DP через ModuleConfig с `spec.enabled: false`. Если компонентом должна управлять DP, перед установкой отключите или удалите его внешний аналог.

## Подготовка kubeconfig для контейнера установщика

Установщик запускается внутри Docker. Ему нужна переносимая копия kubeconfig, которая не ссылается на файлы сертификатов и ключей, доступные только на компьютере пользователя.

Создайте такую копию:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" config view --raw --flatten --minify > "$INSTALLER_KUBECONFIG"
chmod 600 "$INSTALLER_KUBECONFIG"
```

Команда не создаёт новые сертификаты. Параметр `--flatten` читает CA, клиентский сертификат и ключ по путям из исходного kubeconfig и встраивает их в новый файл.

Проверьте копию:

```bash
d8 k --kubeconfig="$INSTALLER_KUBECONFIG" auth whoami
d8 k --kubeconfig="$INSTALLER_KUBECONFIG" auth can-i '*' '*' --all-namespaces
```

Вторая команда должна вывести `yes`.

Посмотрите адрес Kubernetes API:

```bash
d8 k --kubeconfig="$INSTALLER_KUBECONFIG" config view --minify -o jsonpath='{.clusters[0].cluster.server}{"\n"}'
```

Этот адрес должен быть доступен из Docker-контейнера. Предпочтительный вариант — доступный по сети адрес Kubernetes API, VPN или адрес балансировщика.

Если указан `https://127.0.0.1:6443`, установщик не сможет использовать его напрямую: внутри контейнера `127.0.0.1` указывает на сам контейнер. Сначала организуйте доступ контейнера к Kubernetes API и только затем продолжайте установку.

## Создание файла конфигурации

Создайте файл `$CONFIG_FILE` со следующим содержимым и замените `example.com` на свой домен:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Managed
    releaseChannel: EarlyAccess
    logLevel: Info
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings:
    modules:
      publicDomainTemplate: "%s.example.com"
```

Домен в [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) не должен совпадать с доменом, указанным в [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain), или быть его поддоменом. Перед использованием шаблона настройте DNS-сервисы в сетях, где расположены узлы кластера, и в сетях, из которых клиенты обращаются к веб-интерфейсам сервисов платформы.

Если на узлах есть нестандартные taints и компоненты DP должны на них запускаться, добавьте соответствующие значения в [`global.spec.settings.modules.placement.customTolerationKeys`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-placement-customtolerationkeys). Не добавляйте примерный taint, если его нет в кластере.

Проверьте файл:

```bash
yq eval-all '.' "$CONFIG_FILE" >/dev/null && echo "YAML OK"
grep -n $'\t' "$CONFIG_FILE"
```

Первая команда должна вывести `YAML OK`, вторая — ничего.

## Запуск установщика Deckhouse Platform Open

Тег установщика должен соответствовать `releaseChannel` в конфигурации. Для `EarlyAccess` используется тег `early-access`.

Проверьте наличие файлов:

```bash
ls -l "$CONFIG_FILE" "$INSTALLER_KUBECONFIG"
```

Запустите установщик:

```bash
docker run --pull=always -it -v "$CONFIG_FILE:/config.yml:ro" -v "$INSTALLER_KUBECONFIG:/kubeconfig:ro" registry.deckhouse.ru/deckhouse/ce/install:early-access bash
```

Путь образа установщика пока использует обозначение `ce` для Deckhouse Platform Open.

Внутри открывшегося контейнера запустите:

```bash
dhctl bootstrap-phase install-deckhouse --kubeconfig=/kubeconfig --config=/config.yml
```

Не закрывайте терминал до завершения бутстрапа. Установка может занимать от 5 до 30 минут.

## Наблюдение за установкой

В отдельном терминале перейдите в тот же рабочий каталог и снова задайте переменные из раздела «Настройка рабочих путей». Выполните команду:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" -n d8-system get deployment,replicaset,pods -w
```

Если под `deckhouse` не создаётся, посмотрите события:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" -n d8-system get events --sort-by=.metadata.creationTimestamp
```

Ошибки `ImagePullBackOff`, `ErrImagePull`, `401 Unauthorized` или `403 Forbidden` обычно означают проблему с адресом хранилища образов контейнеров, доступом к нему, DNS или маршрутизацией.

Для диагностики конкретного пода используйте:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" -n <NAMESPACE> describe pod <POD_NAME>
```

## Проверка результата установки

Дождитесь готовности основного Deployment:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" -n d8-system rollout status deployment/deckhouse --timeout=10m
d8 k --kubeconfig="$ADMIN_KUBECONFIG" -n d8-system get deployment,pods -o wide
```

Проверьте модули:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get modules -o wide
```

У включённых модулей ожидаются `PHASE: Ready`, `ENABLED: True` и `READY: True`.

Статуса Module недостаточно: он может быть `Ready`, даже если отдельный workload модуля не был создан или перезапускается. Проверьте реальные ресурсы:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get deployment,statefulset,daemonset -A
```

У DaemonSet значения `DESIRED`, `CURRENT` и `READY` должны совпадать. У Deployment и StatefulSet ожидаемое количество реплик должно быть готово.

Найдите поды, которые не находятся в фазе `Running` или `Succeeded`:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get pods -A --field-selector='status.phase!=Running,status.phase!=Succeeded'
```

Проверьте актуальные предупреждения:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get events -A --field-selector=type=Warning --sort-by=.metadata.creationTimestamp
```

Старое предупреждение само по себе не означает текущую неисправность. Учитывайте время события, число повторов и состояние связанного ресурса.

### Проверка, что DP не управляет компонентами Talos

Проверьте, что модули DP, которые могут управлять компонентами Talos, остаются выключенными.

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get modules control-plane-manager node-manager terraform-manager cni-cilium kube-dns kube-proxy registry-packages-proxy -o wide
```

Все перечисленные модули должны иметь `ENABLED: False`.

Проверьте модули облачных провайдеров:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get modules -o wide | grep -E '(^NAME|^cloud-provider-)'
```

Все найденные модули `cloud-provider-*` должны иметь `ENABLED: False`.

Повторно проверьте исходные компоненты:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" -n kube-system get pods -o wide

d8 k --kubeconfig="$ADMIN_KUBECONFIG" get nodes -o wide
```

Все Talos-узлы должны оставаться `Ready`, а исходные CNI, CoreDNS и компоненты control plane — продолжать работать. Если kube-proxy использовался до установки DP, он также должен продолжать работать.

## Критерии успешной установки

Установка считается успешной, если одновременно выполняются следующие условия:

- все Talos-узлы остались `Ready`;
- исходные CNI и CoreDNS продолжают работать, а kube-proxy — если он использовался до установки DP;
- Deployment `deckhouse` готов;
- включённые модули имеют `READY: True`;
- реальные Deployment, StatefulSet и DaemonSet модулей готовы;
- lifecycle-модули DP остаются выключенными;
- административный доступ через Talos admin kubeconfig сохранён.

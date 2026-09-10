---
title: Установка DKP в существующий Talos-кластер
permalink: ru/guides/talos-existing-cluster.html
description: Руководство по установке Deckhouse Kubernetes Platform в существующий Talos-кластер.
lang: ru
layout: sidebar-guides
---

Эта инструкция подходит для ситуации, когда Talos-кластер уже создан и работает: control plane запущен, worker-узлы присоединены, CNI установлен, а Kubernetes API доступен через `kubectl`.

Deckhouse устанавливается поверх готового Kubernetes-кластера в режиме existing cluster. В примере используется Community Edition, release channel `EarlyAccess` и bundle `Managed`.

В этой схеме:

- Talos продолжает управлять ОС, MachineConfig, kubelet, containerd, etcd, control plane, Kubernetes PKI и обновлением Kubernetes;
- существующий CNI продолжает отвечать за сеть Pod;
- внешний инфраструктурный провайдер или пользователь продолжает создавать и удалять машины;
- Deckhouse устанавливает и обновляет платформенные модули, но не управляет Talos и жизненным циклом узлов.

> **Важно.** Значение `bundle` выбирается во время установки и впоследствии не изменяется. Нельзя установить `Managed`, а затем обычным patch переключить его на `Minimal` или `Default`.

## 1. Что понадобится

На компьютере, с которого запускается установка, нужны:

- Docker;
- `kubectl`;
- `yq` для проверки YAML;
- административный Kubernetes kubeconfig Talos-кластера;
- доступ к Kubernetes API;
- HTTPS-доступ к `registry.deckhouse.ru` с компьютера и узлов кластера;
- `talosctl` и `talosconfig`, если административный Kubernetes kubeconfig ещё не получен.

SSH к Talos-узлам не нужен: installer работает через Kubernetes API.

Перед установкой рекомендуется сделать snapshot etcd средствами Talos и сохранить исходные `talosconfig` и Kubernetes kubeconfig.

## 2. Один раз задаём рабочие пути

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
| `ADMIN_KUBECONFIG` | Административный Kubernetes kubeconfig для команд `kubectl` на компьютере |
| `INSTALLER_KUBECONFIG` | Переносимая копия административного kubeconfig для Docker-контейнера |
| `CONFIG_FILE` | Конфигурация установки Deckhouse |

Если вы открыли новый терминал, снова перейдите в каталог и повторите блок с четырьмя переменными.

## 3. Готовим административный Kubernetes kubeconfig

### Если kubeconfig уже есть

Скопируйте его в рабочий каталог:

```bash
cp /путь/к/существующему/admin-kubeconfig "$ADMIN_KUBECONFIG"
chmod 600 "$ADMIN_KUBECONFIG"
```

Это должен быть Kubernetes kubeconfig для `kubectl`, а не `talosconfig` для `talosctl`.

### Если kubeconfig нужно получить через Talos

Сначала поместите существующий `talosconfig` в рабочий каталог:

```bash
cp /путь/к/существующему/talosconfig "$TALOSCONFIG"
chmod 600 "$TALOSCONFIG"
```

Укажите адрес control-plane-узла:

```bash
CONTROL_PLANE_ADDRESS=<IP-или-DNS-control-plane-узла>
```

Получите административный Kubernetes kubeconfig:

```bash
talosctl kubeconfig "$ADMIN_KUBECONFIG" \
  --talosconfig="$TALOSCONFIG" \
  --nodes="$CONTROL_PLANE_ADDRESS" \
  --merge=false

chmod 600 "$ADMIN_KUBECONFIG"
```

По умолчанию `talosctl` возьмёт Talos API endpoints из текущего контекста `talosconfig`. Если нужен другой endpoint, добавьте:

```text
--endpoints=<доступный-Talos-API-endpoint>
```

Параметр `--force` используйте только при осознанной перезаписи существующего файла.

### Проверяем права

Проверьте, кем Kubernetes видит пользователя:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" auth whoami
```

Для установки нужен стабильный административный доступ. Talos admin kubeconfig обычно использует группу `system:masters`. Пользовательский OIDC kubeconfig для installer нежелателен: после включения модулей авторизации Deckhouse права такого пользователя на системные namespace могут измениться.

Проверьте основные разрешения:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  auth can-i '*' '*' --all-namespaces

kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  auth can-i create customresourcedefinitions.apiextensions.k8s.io

kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  auth can-i create clusterroles.rbac.authorization.k8s.io
```

Все три команды должны вывести `yes`.

## 4. Проверяем исходный кластер

Проверьте узлы:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get nodes -o wide
```

Все узлы должны быть в состоянии `Ready`.

Проверьте Kubernetes API:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get --raw='/readyz?verbose'
```

В конце ответа должно быть `readyz check passed`.

Проверьте системные Pod:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n kube-system get pods -o wide
```

До установки Deckhouse уже должны работать:

- CNI;
- CoreDNS;
- kube-proxy, если он используется выбранной сетевой схемой;
- компоненты control plane.

Также убедитесь, что версия Kubernetes поддерживается выбранной версией DKP.

## 5. Проверяем границы ответственности

Deckhouse не должен одновременно с Talos или внешним инфраструктурным провайдером управлять одними и теми же компонентами.

| Компонент | Владелец после установки |
| --- | --- |
| Talos OS и MachineConfig | Talos |
| etcd и Kubernetes control plane | Talos |
| Kubernetes PKI | Talos |
| kubelet и containerd | Talos |
| CNI | Уже установленный внешний CNI |
| CoreDNS и kube-proxy | Существующий кластер |
| Создание и удаление машин | Внешний инфраструктурный провайдер или пользователь |
| Платформенные модули | Deckhouse |

В этом сценарии следующие модули Deckhouse должны оставаться выключенными:

- `control-plane-manager`;
- `node-manager`;
- `terraform-manager`;
- `cni-cilium`;
- `kube-dns`;
- `kube-proxy`;
- cloud-provider-модули;
- `registry-packages-proxy`.

Если в Talos-кластере уже установлен Cilium, включать Deckhouse `cni-cilium` нельзя: два оператора не должны одновременно управлять одним CNI.

Bundle `Managed` включает ingress, cert-manager, local-path-provisioner, VPA, мониторинг и модули авторизации. До установки проверьте, нет ли в кластере их внешних аналогов:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get storageclass
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get ingressclass
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get deployments -A
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get crd
```

Если компонент уже установлен, заранее определите единственного владельца. Не запускайте одновременно два ingress-controller, два cert-manager или два VPA.

## 6. Готовим kubeconfig для installer-контейнера

Installer запускается внутри Docker. Ему нужна переносимая копия kubeconfig, которая не ссылается на файлы сертификатов и ключей, доступные только на компьютере пользователя.

Создайте такую копию:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  config view \
  --raw \
  --flatten \
  --minify \
  > "$INSTALLER_KUBECONFIG"

chmod 600 "$INSTALLER_KUBECONFIG"
```

Команда не создаёт новые сертификаты. Параметр `--flatten` читает CA, клиентский сертификат и ключ по путям из исходного kubeconfig и встраивает их в новый файл.

Проверьте копию:

```bash
kubectl --kubeconfig="$INSTALLER_KUBECONFIG" auth whoami

kubectl --kubeconfig="$INSTALLER_KUBECONFIG" \
  auth can-i '*' '*' --all-namespaces
```

Вторая команда должна вывести `yes`.

Посмотрите адрес Kubernetes API:

```bash
kubectl --kubeconfig="$INSTALLER_KUBECONFIG" \
  config view --minify \
  -o jsonpath='{.clusters[0].cluster.server}{"\n"}'
```

Этот адрес должен быть доступен из Docker-контейнера. Предпочтительный вариант — доступный по сети адрес Kubernetes API, VPN или адрес балансировщика.

Если указан `https://127.0.0.1:6443`, installer не сможет использовать его напрямую: внутри контейнера `127.0.0.1` указывает на сам контейнер. Сначала организуйте доступ контейнера к Kubernetes API и только затем продолжайте установку.

## 7. Создаём `config.yml`

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

`publicDomainTemplate` не должен совпадать с Kubernetes `clusterDomain`.

Если на узлах есть нестандартные taint и компоненты Deckhouse должны на них запускаться, добавьте соответствующие значения в `global.spec.settings.modules.placement.customTolerationKeys`. Не добавляйте примерный taint, если его нет в кластере.

Проверьте файл:

```bash
yq eval-all '.' "$CONFIG_FILE" >/dev/null && echo "YAML OK"
grep -n $'\t' "$CONFIG_FILE"
```

Первая команда должна вывести `YAML OK`, вторая — ничего.

## 8. Запускаем официальный CE installer

Тег installer должен соответствовать `releaseChannel` в конфигурации. Для `EarlyAccess` используется тег `early-access`.

Проверьте наличие файлов:

```bash
ls -l "$CONFIG_FILE" "$INSTALLER_KUBECONFIG"
```

Запустите installer:

```bash
docker run --pull=always -it \
  -v "$CONFIG_FILE:/config.yml:ro" \
  -v "$INSTALLER_KUBECONFIG:/kubeconfig:ro" \
  registry.deckhouse.ru/deckhouse/ce/install:early-access \
  bash
```

Внутри открывшегося контейнера запустите:

```bash
dhctl bootstrap-phase install-deckhouse \
  --kubeconfig=/kubeconfig \
  --config=/config.yml
```

Не закрывайте терминал до завершения bootstrap. Обычно установка занимает от 5 до 30 минут.

## 9. Наблюдаем за установкой

В отдельном терминале перейдите в тот же рабочий каталог и снова задайте переменные из раздела 2. Затем следите за Deckhouse:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system get deployment,replicaset,pods -w
```

Если Pod не создаётся, посмотрите события:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system get events \
  --sort-by=.metadata.creationTimestamp
```

Ошибки `ImagePullBackOff`, `ErrImagePull`, `401 Unauthorized` или `403 Forbidden` обычно означают проблему с адресом registry, доступом к нему, DNS или маршрутизацией.

Для диагностики конкретного Pod используйте:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n <NAMESPACE> describe pod <POD_NAME>
```

## 10. Проверяем результат

Дождитесь готовности основного Deployment:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system rollout status deployment/deckhouse \
  --timeout=10m

kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system get deployment,pods -o wide
```

Проверьте модули:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get modules -o wide
```

У включённых модулей ожидаются `PHASE: Ready`, `ENABLED: True` и `READY: True`.

Статуса `Module` недостаточно: он может быть `Ready`, даже если отдельный workload модуля не был создан или перезапускается. Проверьте реальные ресурсы:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  get deployment,statefulset,daemonset -A
```

У DaemonSet значения `DESIRED`, `CURRENT` и `READY` должны совпадать. У Deployment и StatefulSet ожидаемое количество реплик должно быть готово.

Найдите остальные проблемные Pod:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  get pods -A \
  --field-selector='status.phase!=Running,status.phase!=Succeeded'
```

Проверьте актуальные предупреждения:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  get events -A \
  --field-selector=type=Warning \
  --sort-by=.metadata.creationTimestamp
```

Старое предупреждение само по себе не означает текущую неисправность. Учитывайте время события, число повторов и состояние связанного ресурса.

### Проверяем, что Deckhouse не забрал управление Talos

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get modules \
  control-plane-manager \
  node-manager \
  terraform-manager \
  cni-cilium \
  kube-dns \
  kube-proxy \
  registry-packages-proxy \
  -o wide
```

В этом сценарии они должны иметь `ENABLED: False`.

Повторно проверьте исходные компоненты:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n kube-system get pods -o wide

kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  get nodes -o wide
```

Все Talos-узлы должны оставаться `Ready`, а исходные CNI, CoreDNS, kube-proxy и компоненты control plane — продолжать работать.

## 11. Критерии успешной установки

Установка считается успешной, если одновременно выполняются следующие условия:

- все Talos-узлы остались `Ready`;
- исходные CNI, CoreDNS и kube-proxy продолжают работать;
- Deployment Deckhouse готов;
- включённые модули имеют `READY: True`;
- реальные Deployment, StatefulSet и DaemonSet модулей готовы;
- lifecycle-модули Deckhouse остаются выключенными;
- административный доступ через Talos admin kubeconfig сохранён.

## Полезные ссылки

- [Установка Deckhouse в существующий кластер](https://deckhouse.ru/products/kubernetes-platform/gs/existing/step2.html)
- [Настройки модуля deckhouse](https://deckhouse.ru/modules/deckhouse/configuration.html)
- [Bundle и управление модулями](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/admin/configuration/)
- [Изменение Talos MachineConfig с помощью patches](https://docs.siderolabs.com/talos/v1.13/configure-your-talos-cluster/system-configuration/patching)

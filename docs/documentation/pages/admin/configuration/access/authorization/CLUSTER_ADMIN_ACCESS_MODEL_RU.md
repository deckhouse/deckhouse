---
title: "Модель административного доступа к кластеру"
permalink: ru/admin/configuration/access/authorization/cluster-admin-access-model.html
description: "Модель административного доступа к кластеру Deckhouse Kubernetes Platform"
lang: ru
---

Deckhouse Kubernetes Platform поддерживает размещение нескольких файлов kubeconfig на master-узлах (поддержка реализуется модулем [`control-plane-manager`](/modules/control-plane-manager/)). Понимание их назначения важно для безопасного администрирования кластера.

## Файлы kubeconfig на master-узлах

На master-узлах размещаются следующие файлы kubeconfig:

| Файл | Идентификация | Назначение |
| --- | --- | --- |
| `/etc/kubernetes/admin.conf` | `kubernetes-admin` (группа `kubeadm:cluster-admins`) | Машинный kubeconfig для внутренних операций kubeadm (join, обновление). При включённом модуле [`user-authz`](/modules/user-authz/) RBAC использует `user-authz:cluster-admin` и дополнительную ClusterRole. При выключенном `user-authz` группа привязана к встроенной роли `cluster-admin` |
| `/etc/kubernetes/super-admin.conf` | `kubernetes-super-admin` (группа `system:masters`) | Аварийный доступ (break-glass). Обходит RBAC полностью. Ограничьте доступ к файлу сценариями восстановления |
| `/etc/kubernetes/controller-manager.conf` | `system:kube-controller-manager` | Используется kube-controller-manager |
| `/etc/kubernetes/scheduler.conf` | `system:kube-scheduler` | Используется kube-scheduler |

## Административный доступ на основе RBAC

Начиная с Kubernetes 1.29, kubeadm генерирует `admin.conf` с группой `kubeadm:cluster-admins` вместо `system:masters`. Это обеспечивает управляемый через RBAC административный доступ, который может быть отозван путём удаления ClusterRoleBinding `kubeadm:cluster-admins` (или нескольких привязок).

Если модуль [`user-authz`](/modules/user-authz/) **выключен**, DKP привязывает группу `kubeadm:cluster-admins` к встроенной роли `cluster-admin` с wildcard-правами (как в обычном кластере kubeadm без дополнительной настройки RBAC).

Если модуль `user-authz` **включён**, группа привязывается к `user-authz:cluster-admin`, а вторая ClusterRoleBinding добавляет роль `d8:control-plane-manager:admin-kubeconfig-supplement` (правила сверх высокоуровневой роли, например для сертификатов и компонентов control plane). Вместе они заменяют одну wildcard-роль `cluster-admin` для этой идентичности. Для полного неограниченного доступа используйте `super-admin.conf`.

## Рекомендуемый административный доступ

Если модуль [`user-authn`](/modules/user-authn/) включён, используйте персонализированный kubeconfig на основе OIDC, получаемый через kubeconfig-генератор. Это обеспечивает индивидуальную ответственность и журнал аудита.

Если `user-authn` отключён, администраторы могут явно использовать admin kubeconfig на master-узле:

```bash
d8 k --kubeconfig=/etc/kubernetes/admin.conf <команда>
```

## Символическая ссылка root kubeconfig

По умолчанию модуль [`control-plane-manager`](/modules/control-plane-manager/) создаёт символическую ссылку `/root/.kube/config` → `/etc/kubernetes/admin.conf` на master-узлах, что позволяет root-пользователю запускать `d8 k` без указания `--kubeconfig`.

Если модуль `user-authz` включён, это поведение можно отключить, задав [`rootKubeconfigSymlink: false`](modules/control-plane-manager/configuration.html#parameters-rootkubeconfigsymlink) в конфигурации модуля `control-plane-manager`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 2
  enabled: true
  settings:
    rootKubeconfigSymlink: false
```

Если модуль `user-authz` выключен, `control-plane-manager` не использует параметр `rootKubeconfigSymlink` и сохраняет поведение по умолчанию (симлинк создаётся).

При отключении симлинка (при включённом `user-authz`) ссылка удаляется, если она указывала на `admin.conf`. Используйте персонализированные учётные данные или явно указывайте `--kubeconfig`.

## Усиление безопасности

Модуль `control-plane-manager` автоматически ограничивает права доступа к файлам `admin.conf` и `super-admin.conf` до `0600` (чтение/запись только для владельца) при каждом цикле согласования. Это предотвращает несанкционированный доступ к этим конфиденциальным учётным данным.

## Аварийный доступ (break-glass) к кластеру

В экстренных ситуациях (например, при ошибках в конфигурации RBAC, отказе вебхуков) используйте `super-admin.conf`:

```bash
d8 k --kubeconfig=/etc/kubernetes/super-admin.conf <команда>
```

Эти учётные данные обходят все проверки RBAC. Используйте их только в крайнем случае и ограничьте круг лиц с доступом к файлу `super-admin.conf`.

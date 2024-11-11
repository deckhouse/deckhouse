---
title: "Управление DNS"
permalink: en/virtualization-platform/documentation/admin/platform-management/traffic-control/dns.html
---

Модуль устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes.

Внимание! Модуль удаляет ранее установленные kubeadm’ом Deployment, ConfigMap и RBAC для CoreDNS.

Чтобы включить модуль static-routing-manager с настрйоками по умолчанию, примените следующий ресурс `ModuleConfig`:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  enabled: true
EOF
```

Пример конфигурации модуля с помощью ресурса `ModuleConfig`:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  version: 1
  enabled: true
  settings:
    # Список IP-адресов рекурсивных DNS-серверов, которые CoreDNS будет использовать для разрешения внешних доменов.
    # По умолчанию используется список из /etc/resolv.conf.
    upstreamNameservers:
      - 8.8.8.8
      - 8.8.4.4
    # Статический список хостов в стиле /etc/hosts:
    hosts:
      - domain: one.example.com
        ip: 192.168.0.1
      - domain: two.another.example.com
        ip: 10.10.0.128
    # Список дополнительных зон для обслуживания CoreDNS.
    stubZones:
      - zone: consul.local
        upstreamNameservers:
          - 10.150.0.1
    # Список альтернативных доменов кластера, разрешаемых наравне с global.discovery.clusterDomain.
    clusterDomainAliases:
      - foo.bar
      - baz.qux
EOF
```

## Изменение домена кластера

Чтобы поменять домен кластера с минимальным простоем, добавьте новый домен и сохраните предыдущий. 

1. Для этого измените конфигурацию параметров в настроках модуля control-plane-manager:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  enabled: true
  settings:
    apiserver:
      certSANs:
       - kubernetes.default.svc.<старый clusterDomain>
       - kubernetes.default.svc.<новый clusterDomain>
      serviceAccount:
        additionalAPIAudiences:
        - https://kubernetes.default.svc.<старый clusterDomain>
        - https://kubernetes.default.svc.<новый clusterDomain>
        additionalAPIIssuers:
        - https://kubernetes.default.svc.<старый clusterDomain>
        - https://kubernetes.default.svc.<новый clusterDomain>
```

2. Затем укажите список альтернативных доменов кластера в настройках модуля kube-dns:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: kube-dns
   spec:
     version: 1
     enabled: true
     settings:
       clusterDomainAliases:
         - <старый clusterDomain>
         - <новый clusterDomain>
   ```

3. Дождитесь перезапуска `kube-apiserver`.
4. Поменяйте `clusterDomain` на новый в `dhctl config edit cluster-configuration`.

**Важно!** Если версия вашего Kubernetes 1.20 и выше, контроллеры для работы с API-server гарантированно используют [расширенные токены для ServiceAccount'ов](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection). Это означает, что каждый такой токен содержит дополнительные поля `iss:` и `aud:`, которые включают в себя старый `clusterDomain` (например, `"iss": "https://kubernetes.default.svc.cluster.local"`).
При смене `clusterDomain` API-server начнет выдавать токены с новым `service-account-issuer`, но благодаря произведенной конфигурации `additionalAPIAudiences` и `additionalAPIIssuers` по-прежнему будет принимать старые токены. По истечении 48 минут (80% от 3607 секунд) Kubernetes начнет обновлять выпущенные токены, при обновлении будет использован новый `service-account-issuer`. Через 90 минут (3607 секунд и немного больше) после перезагрузки kube-apiserver можете удалить конфигурацию `serviceAccount` из конфигурации `control-plane-manager`.

**Важно!** Если вы используете модуль istio, после смены `clusterDomain` обязательно потребуется рестарт всех прикладных подов под управлением Istio.

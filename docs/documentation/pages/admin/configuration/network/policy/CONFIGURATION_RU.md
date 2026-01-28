---
title: "Настройка сетевых политик"
permalink: ru/admin/configuration/network/policy/configuration.html
description: |
  Настройка сетевых политик Deckhouse Kubernetes Platform. Условия для включения режимов туннелирования трафика подов.
lang: ru
search: network policy configuration, network policies, traffic tunneling, pod traffic, network security, настройка сетевых политик, сетевые политики, сетевая безопасность
---

Если на уровне инфраструктуры, где работает Deckhouse Kubernetes Platform, есть требования для ограничения сетевого взаимодействия между серверами, то необходимо соблюсти следующие условия:

* Включен режим туннелирования трафика между подами ([настройки](/modules/cni-cilium/configuration.html#parameters-tunnelmode) для CNI Cilium, [настройки](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) для CNI Flannel).
* Разрешена передача трафика между [`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr), инкапсулированного внутри VXLAN (если выполняется инспектирование и фильтрация трафика внутри VXLAN-туннеля).
* В случае необходимости интеграции с внешними системами (например, LDAP, SMTP или прочие внешние API), с ними разрешено сетевое взаимодействие.
* Локальное сетевое взаимодействие полностью разрешено в рамках каждого отдельно взятого узла кластера.
* Разрешено взаимодействие между узлами по портам, приведенным в [списке сетевого взаимодействия компонентов платформы](../../../../reference/network_interaction.html). Обратите внимание, что большинство портов входит в диапазон 4200-4299. При добавлении новых компонентов платформы им будут назначаться порты из этого диапазона (при наличии возможности).

## Настройка сетевых политик стандартными средствами Kubernetes

<!-- пример взят из обучающих материалов -->

### Пример настроек сетевой политики

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-network-policy
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: db
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - ipBlock:
        cidr: 172.17.0.0/16
        except:
        - 172.17.1.0/24
    - namespaceSelector:
        matchLabels:
          project: myproject
    - podSelector:
        matchLabels:
          role: frontend
    ports:
    - protocol: TCP
      port: 6379
  egress:
  - to:
    - ipBlock:
        cidr: 10.0.0.0/24
    ports:
    - protocol: TCP
      port: 5978
```

## Настройка сетевых политик на уровне всего кластера с помощью CiliumClusterwideNetworkPolicy

Для определения сетевых политик на уровне всего кластера в Deckhouse Kubernetes Platform можно использовать объекты CiliumClusterwideNetworkPolicy модуля [`cni-cilium`](/modules/cni-cilium/).

<!-- перенесено с некоторыми изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/cni-cilium/#%D0%B8%D1%81%D0%BF%D0%BE%D0%BB%D1%8C%D0%B7%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D0%B5-ciliumclusterwidenetworkpolicies -->

{% alert level="danger" %}
Использование объектов CiliumClusterwideNetworkPolicy без включения параметра `policyAuditMode` в настройках модуля `cni-cilium` может привести к некорректной работе control plane или потере доступа ко всем узлам кластера по SSH.
{% endalert %}

Для использования объектов CiliumClusterwideNetworkPolicy выполните следующие шаги:

1. Примените первичный набор объектов CiliumClusterwideNetworkPolicy. Для этого в настройки модуля `cni-cilium` добавьте конфигурационную опцию [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) со значением `true`.

   Опция `policyAuditMode` может быть удалена после применения всех объектов CniliumClusterwideNetworkPolicy и проверки корректности их работы в Hubble UI.

1. Примените правило политики сетевой безопасности:

   ```yaml
   apiVersion: "cilium.io/v2"
   kind: CiliumClusterwideNetworkPolicy
   metadata:
     name: "allow-control-plane-connectivity"
   spec:
     ingress:
     - fromEntities:
       - kube-apiserver
     nodeSelector:
       matchLabels:
         node-role.kubernetes.io/control-plane: ""
   ```

В случае, если объекты CiliumClusterwideNetworkPolicy не применяются, control plane может некорректно работать до одной минуты во время перезагрузки подов `cilium-agent`. Это происходит из-за [сброса Conntrack-таблицы](https://github.com/cilium/cilium/issues/19367). Привязка к entity `kube-apiserver` позволяет избежать проблемы.

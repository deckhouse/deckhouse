---
title: "Сетевые политики"
permalink: ru/admin/network/network-policies-overview.html
lang: ru
---

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/network_security_setup.html -->

Если на площадке, где работает Deckhouse Kubernetes Platform, есть требования для ограничения сетевого взаимодействия между серверами на уровне инфраструктуры, то необходимо соблюсти следующие условия:

* Включен режим туннелирования трафика между подами ([настройки](modules/cni-cilium/configuration.html#parameters-tunnelmode) для CNI Cilium, [настройки](modules/cni-flannel/configuration.html#parameters-podnetworkmode) для CNI Flannel).
* Разрешена передача трафика между [podSubnetCIDR](installing/configuration.html#clusterconfiguration), инкапсулированного внутри VXLAN (если выполняется инспектирование и фильтрация трафика внутри VXLAN-туннеля).
* В случае необходимости интеграции с внешними системами (например, LDAP, SMTP или прочие внешние API), с ними разрешено сетевое взаимодействие.
* Локальное сетевое взаимодействие полностью разрешено в рамках каждого отдельно взятого узла кластера.
* Разрешено взаимодействие между узлами по портам, приведенным в таблицах на текущей странице. Обратите внимание, что большинство портов входит в диапазон 4200-4299. При добавлении новых компонентов платформы им будут назначаться порты из этого диапазона (при наличии возможности).

{% include network_security_setup.liquid %}

<!-- пример взят из обучающих материалов -->

## Пример настроек сетевой политики

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

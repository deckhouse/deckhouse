---
title: "Сетевые политики"
permalink: ru/virtualization-platform/documentation/user/network/network-policies.html
lang: ru
---

## Основные положения

Для управления входящим и исходящим трафиком виртуальных машин на уровне 3 или 4 модели OSI используются стандартные сетевые политики Kubernetes. Более подробно об этом можно прочитать в официальной документации [Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/).  

Есть два основных типа управления трафиком, входящий и исходящий:

- Ingress – входящий трафик;
- Egress – исходящий трафик.

Для управления внутрикластерным трафиком рекомендуется использовать `podSelector` и `namespaceSelector`, а для сетевого взаимодействия за пределами кластера – `ipBlock`.
Правила сетевых политик применяются одновременно, по принципу их складывания, для всех виртуальных машин, которые соответствуют указанным меткам.

Дальнейшие примеры будут показаны на примере проекта `test-project` с двумя виртуальными машинами в пространстве имён `test-project`.

По умолчанию входящий и исходящий трафик не ограничены:

```shell
d8 k get vm -n test-project
```

Пример вывода:

```console
NAME   PHASE     NODE           IPADDRESS     AGE
vm-a   Running   virtlab-2      10.66.20.70   5m
vm-b   Running   virtlab-1      10.66.20.71   5m
```

Виртуальные машины имеют соответствующие им метки.

```shell
d8 k get vm -n test-project -o yaml | less
```

Пример вывода:

```yaml
- apiVersion: virtualization.deckhouse.io/v1alpha2
  kind: VirtualMachine
  metadata:
    labels:
      vm: a
    name: vm-a
    namespace: test-project
- apiVersion: virtualization.deckhouse.io/v1alpha2
  kind: VirtualMachine
  metadata:
    labels:
      vm: b
    name: vm-b
    namespace: test-project
```

## Изоляция всего входящего трафика виртуальной машины

Сетевая политика, ограничивающая весь входящий трафик для виртуальных машин с меткой `vm-a`, в пространстве имён `test-project`:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vm-a-deny-ingress
  namespace: test-project
spec:
  podSelector:
    matchLabels:
      vm: a
  policyTypes:
    - Ingress
```

Тип политики (policy type) Ingress означает, что будут применены правила для входящего трафика. Так как никаких Ingress правил в спецификации не указано, то будет ограничен весь входящий трафик.

По такому же принципу можно ограничить и исходящий трафик, добавив Egress в блок `spec.policyTypes`.

```yaml
policyTypes:
  - Egress
  - Ingress
```

## Разрешение входящего трафика между виртуальными машинами

Сетевая политика, разрешающая входящий трафик от виртуальных машин с меткой `vm-b`, до виртуальных машин с меткой `vm-a`:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-ingress-from-vm-b-to-vm-a
  namespace: test-project
spec:
  podSelector:
    matchLabels:
      vm: a
  ingress:
    - from:
      - podSelector:
          matchLabels:
            vm: b
  policyTypes:
    - Ingress
```

С помощью `spec.podSelector` для всех виртуальных машин с меткой `vm: a` применяется сетевая политика с типом Ingress. В спецификации `spec.ingress` указано правило, которое разрешает входящий трафик `from` из виртуальных машин с меткой `vm: b`.

## Разрешение исходящего трафика виртуальной машины за пределы кластера

Сетевая политика, разрешающая исходящий трафик от виртуальных машин с меткой `vm-a` до внешнего адреса 8.8.8.8:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-egress-from-vm-a-to-8-8-8-8
  namespace: test-project
spec:
  podSelector:
    matchLabels:
      vm: a
  egress:
    - to:
      - ipBlock:
          cidr: 8.8.8.8/32
        ports:
          - protocol: TCP
            port: 53
  policyTypes:
    - Egress
```

Тип политики (policy type) указывает на то, что будут применены правила исходящего трафика в спецификации `spec.egress`. Также указаны протокол `TCP` и порт `53`, на который разрешён трафик.

Порты могут быть указаны в виде диапазона с помощью дополнительного поля `endPort` в блоке `ports`.

```yaml
ports:
  - protocol: TCP
    port: 32000
    endPort: 32768
```

## Разрешение входящего трафика между пространствами имён

Сетевая политика разрешает входящий трафик до виртуальных машин с меткой `vm: a` из пространства имён `another-project`, которое имеет соответствующую метку `kubernetes.io/metadata.name: another-project`.

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-ingress-from-namespace-another-project-to-vm-a
  namespace: test-project
spec:
  podSelector:
    matchLabels:
      vm: a
  ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: another-project
  policyTypes:
    - Ingress
```

## Полезные ссылки

С полным описанием спецификации сетевых политик можно ознакомиться в документации:

- [https://kubernetes.io/docs/concepts/services-networking/network-policies](https://kubernetes.io/docs/concepts/services-networking/network-policies).
- [https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#networkpolicy-v1-networking-k8s-io](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#networkpolicy-v1-networking-k8s-io).
  
  где `1.31` — версия Kubernetes релиза. При необходимости укажите поддерживаемую в вашем кластере версию.

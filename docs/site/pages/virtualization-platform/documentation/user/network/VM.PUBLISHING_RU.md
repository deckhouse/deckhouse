---
title: "Публикация виртуальных машины"
permalink: ru/virtualization-platform/documentation/user/network/vm-publishing.html
lang: ru
---

## Публикация виртуальных машин с использованием сервисов

Достаточно часто возникает необходимость сделать так, чтобы доступ к этим виртуальным машинам был возможен извне, например, для публикации каких-либо сервисов или удалённого администрирования. Для этих целей мы можем использовать сервисы, которые обеспечивают маршрутизацию трафика из внешней сети к внутренним ресурсам кластера. Рассмотрим несколько вариантов.

Предварительно, проставьте на ранее созданной вм следующие лейблы:

```bash
d8 k label vm linux-vm app=nginx
# virtualmachine.virtualization.deckhouse.io/linux-vm labeled
```

### Публикация сервисов виртуальной машины с использованием сервиса с типом NodePort

Сервис `NodePort` открывает определённый порт на всех узлах кластера, перенаправляя трафик на заданный внутренний порт сервиса.

Создайте следующий сервис:

```yaml
d8 k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: linux-vm-nginx-nodeport
spec:
  type: NodePort
  selector:
    # лейбл по которому сервис определяет на какую виртуальную машину направлять трафик
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
      nodePort: 31880
EOF
```

![](images/lb-nodeport.ru.png)

В данном примере будет создан сервис с типом `NodePort`, который открывает внешний порт 31880 на всех узлах вашего кластера. Этот порт будет направлять входящий трафик на внутренний порт 80 виртуальной машины, где запущено приложение Nginx.

### Публикация сервисов виртуальной машины с использованием сервиса с типом LoadBalancer

При использовании типа сервиса `LoadBalancer` кластер создаёт внешний балансировщик нагрузки, который распределит входящий трафик по всем экземплярам вашей виртуальной машины.

```yaml
d8 k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: linux-vm-nginx-lb
spec:
  type: LoadBalancer
  selector:
    # лейбл по которому сервис определяет на какую виртуальную машину направлять трафик
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
EOF
```

![](images/lb-loadbalancer.ru.png)

### Публикация сервисов виртуальной машины с использованием Ingress

`Ingress` позволяет управлять входящими HTTP/HTTPS запросами и маршрутизировать их к различным серверам в рамках вашего кластера. Это наиболее подходящий метод, если вы хотите использовать доменные имена и SSL-терминацию для доступа к вашим виртуальным машинам.

Для публикации сервиса виртуальной машины через `Ingress` необходимо создать следующие ресурсы:

Внутренний сервис для связки с `Ingress`. Пример:

```yaml
d8 k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: linux-vm-nginx
spec:
  selector:
    # лейбл по которому сервис определяет на какую виртуальную машину направлять трафик
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
EOF
```

И ресурс `Ingress` для публикации. Пример:

```yaml
d8 k apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: linux-vm
spec:
  rules:
    - host: linux-vm.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: linux-vm-nginx
                port:
                  number: 80
EOF
```

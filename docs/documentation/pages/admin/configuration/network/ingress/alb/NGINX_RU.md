---
title: "ALB средствами NGINX Ingress controller"
permalink: ru/admin/configuration/network/ingress/alb/nginx.html
description: "Настройка балансировщика нагрузки приложения с помощью контроллера NGINX Ingress в Deckhouse Kubernetes Platform. Настройка высокой доступности, терминация SSL и конфигурация маршрутизации трафика."
lang: ru
---

Для реализации ALB средствами [NGINX Ingress controller](https://github.com/kubernetes/ingress-nginx) используется модуль [`ingress-nginx`](/modules/ingress-nginx/).

<!-- Перенесено с небольшими изменениями из https://deckhouse.ru/modules/ingress-nginx/ + надо дополнить примерами? -->

Модуль `ingress-nginx` устанавливает NGINX Ingress controller и управляет им с помощью кастомных ресурсов.
Если узлов для размещения Ingress-контроллера больше одного, он устанавливается в отказоустойчивом режиме, с учётом особенностей инфраструктуры как облачных, так и bare-metal сред, а также различных типов Kubernetes-кластеров.

Поддерживается одновременный запуск нескольких экземпляров Ingress-контроллеров с независимой конфигурацией: одного **основного** и произвольного количества **дополнительных**.
Это, например, позволяет разделять внешние и внутренние (intranet) Ingress-ресурсы приложений.

## Варианты терминации трафика

Трафик к `ingress-nginx` может быть отправлен несколькими способами:

* напрямую, без использования внешнего балансировщика;
* через внешний LoadBalancer, в том числе поддерживаются:
  * Qrator,
  * Cloudflare,
  * AWS LB,
  * GCE LB,
  * ACS LB,
  * Yandex LB,
  * OpenStack LB.

## Терминация HTTPS

Для каждого экземпляра NGINX Ingress Controller можно настраивать политики безопасности HTTPS, включая:

* параметры HSTS;
* набор доступных версий SSL/TLS и протоколов шифрования.

Также модуль интегрирован с модулем [`cert-manager`](/modules/cert-manager/), при взаимодействии с которым возможны автоматический заказ SSL-сертификатов и их дальнейшее использование Ingress-контроллерами.

## Мониторинг и статистика

В этой реализации `ingress-nginx` добавлена система сбора статистики в Prometheus с множеством метрик:

* по длительности времени всего ответа и апстрима отдельно;
* кодам ответа;
* количеству повторов запросов (retry);
* размерам запроса и ответа;
* методам запросов;
* типам `content-type`;
* географии распределения запросов и т. д.

Данные представлены в нескольких разрезах:

* `namespace`;
* `vhost`;
* `ingress`-ресурсы;
* `location` (в nginx).

Все графики сгруппированы в дашборды Grafana. Реализована возможность drill-down: например, при просмотре статистики по `namespace` можно перейти по ссылке на соответствующий дашборд и получить детализированные данные по `vhosts` в этом `namespace` — и далее по иерархии.

## Статистика

### Основные принципы сбора статистики

1. На стадии `log_by_lua_block` для каждого запроса вызывается модуль, который рассчитывает необходимые данные и складывает их в буфер (у каждого NGINX worker свой буфер).
2. На стадии `init_by_lua_block` для каждого NGINX worker запускается процесс, который раз в секунду асинхронно отправляет данные в формате `protobuf` через TCP socket в `protobuf_exporter` (разработка Deckhouse Kubernetes Platform).
3. `protobuf_exporter` запускается sidecar-контейнером в поде с ingress-controller, принимает сообщения в формате `protobuf`, разбирает, агрегирует их по установленным нами правилам и экспортирует в формате для Prometheus.
4. Prometheus каждые 30 секунд собирает метрики как в ingress-controller (там есть небольшое количество нужных метрик), так и `protobuf_exporter`. На основе этих данных строится статистика.

### Состав и представление метрик

У всех собираемых метрик есть служебные лейблы, идентифицирующие экземпляр контроллера: `controller`, `app`, `instance` и `endpoint` (они видны в `/prometheus/targets`).

* Все метрики (кроме geo), экспортируемые через `protobuf_exporter`, представлены в трех уровнях детализации:
  * `ingress_nginx_overall_*` — агрегированные метрики верхнего уровня (без детализации, у всех метрик есть лейблы `namespace`, `vhost` и `content_kind`);
  * `ingress_nginx_detail_*` — кроме лейблов уровня overall, добавляются `ingress`, `service`, `service_port` и `location`;
  * `ingress_nginx_detail_backend_*` — ограниченная часть данных, собирается в разрезе по бэкендам. У этих метрик, кроме лейблов уровня detail, добавляется лейбл `pod_ip`.

* Для уровней overall и detail собираются следующие метрики:
  * `*_requests_total` — общее количества запросов (дополнительные лейблы — `scheme`, `method`);
  * `*_responses_total` — количество ответов (дополнительный лейбл — `status`);
  * `*_request_seconds_{sum,count,bucket}` — гистограмма времени ответа;
  * `*_bytes_received_{sum,count,bucket}` — гистограмма размера запроса;
  * `*_bytes_sent_{sum,count,bucket}` — гистограмма размера ответа;
  * `*_upstream_response_seconds_{sum,count,bucket}` — гистограмма времени ответа upstream-сервиса (при нескольких upstream'ах — суммарное время);
  * `*_lowres_upstream_response_seconds_{sum,count,bucket}` — упрощённая гистограмма (для визуализации; не подходит для расчета квантилей);
  * `*_upstream_retries_{count,sum}` — количество и суммарное число повторных запросов (retry) к бэкенду.

* Для уровня overall собираются следующие метрики:
  * `*_geohash_total` — количество запросов по geohash (дополнительные лейблы — `geohash`, `place`).

* Для уровня detail_backend собираются следующие метрики:
  * `*_lowres_upstream_response_seconds` — упрощённая гистограмма времени ответа для overall и detail;
  * `*_responses_total` — количество ответов (дополнительный лейбл — `status_class`, а не просто `status`);
  * `*_upstream_bytes_received_sum` — суммарный объём данных, полученных от бэкендов.

## Примеры настройки балансировки

<!-- перенесено из https://deckhouse.ru/modules/ingress-nginx/examples.html -->

Для настройки балансировки используйте кастомный ресурс [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller).

### Пример для AWS (Network Load Balancer)

При создании балансировщика используются все доступные в кластере зоны.

В каждой зоне балансировщик получает собственный публичный IP. Если в зоне есть экземпляр с Ingress-контроллером, A-запись с IP-адресом балансировщика из этой зоны автоматически добавляется к доменному имени балансировщика.

Если в зоне не остается экземпляров с Ingress-контроллером, тогда IP автоматически убирается из DNS.

В том случае, если в зоне всего один экземпляр с Ingress-контроллером, при перезапуске пода IP-адрес балансировщика этой зоны временно исключается из DNS.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

### Пример для GCP / Yandex Cloud / Azure

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
```

{% alert level="info" %}
В GCP на узлах необходимо указать аннотацию, которая разрешает принимать подключения на внешние адреса для сервисов с типом NodePort.
{% endalert %}

### Пример для OpenStack

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main-lbwpp
spec:
  inlet: LoadBalancerWithProxyProtocol
  ingressClass: nginx
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
```

### Пример создания внутреннего балансировщика для VK Cloud

Этот пример подходит, когда нужно создать балансировщик только внутри сети облака (без внешнего адреса).

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/openstack-internal-load-balancer: "true"
  nodeSelector:
    node.deckhouse.io/group: worker
```

### Пример для bare metal

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostWithFailover
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

### Пример для bare metal при использовании внешнего балансировщика

Пример подходит при использовании Cloudflare, Qrator, Nginx+, Citrix ADC, Kemp и других внешних балансировщиков.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
    behindL7Proxy: true
```

### Пример для bare metal (MetalLB в режиме BGP LoadBalancer)

{% alert level="info" %}
Доступно только в DKP Enterprise Edition.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

В случае использования MetalLB его speaker-поды должны быть запущены на тех же узлах, что и поды Ingress–контроллера.

Чтобы Ingress-контроллер получал реальные IP-адреса клиентов, его сервис должен быть создан с параметром `externalTrafficPolicy: Local`, исключающим межузловой SNAT. Для соблюдения этого условия MetalLB speaker анонсирует этот Service только с тех узлов, где запущены целевые поды.

Таким образом, для данного примера конфигурация модуля [`metallb`](/modules/metallb/configuration.html) должна быть такой:

```yaml
metallb:
 speaker:
   nodeSelector:
     node-role.deckhouse.io/frontend: ""
   tolerations:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

### Пример для bare metal (балансировщик MetalLB в режиме L2 LoadBalancer)

{% alert level="info" %}Доступно только в Enterprise Edition.{% endalert %}

1. Включите [модуль `metallb`](/modules/metallb/):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: metallb
   spec:
     enabled: true
     version: 2
   ```

1. Создайте [ресурс MetalLoadBalancerClass](/modules/metallb/cr.html#metalloadbalancerclass):

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: MetalLoadBalancerClass
   metadata:
     name: ingress
   spec:
     addressPool:
       - 192.168.2.100-192.168.2.150
     isDefault: false
     nodeSelector:
       node-role.kubernetes.io/loadbalancer: "" # Cелектор узлов-балансировщиков.
     type: L2
   ```

1. Создайте [ресурс IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: IngressNginxController
   metadata:
     name: main
   spec:
     ingressClass: nginx
     inlet: LoadBalancer
     loadBalancer:
       loadBalancerClass: ingress
       annotations:
         # Количество адресов, которые будут выделены из пула, описанного в MetalLoadBalancerClass.
         network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
   ```

Платформа создаст сервис с типом LoadBalancer, которому будет присвоено заданное количество адресов:

```shell
d8 k -n d8-ingress-nginx get svc
```

Пример вывода:

```console
NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)                      AGE
main-load-balancer     LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30689/TCP,443:30668/TCP   11s
```

### Пример разделения доступа между публичной и административной зонами

Во многих приложениях один и тот же backend обслуживает как публичную часть, так и административный интерфейс. Например:

- `https://example.com` — публичная зона;
- `https://admin.example.com` — административная зона, к которой доступ должен быть ограничен (`ACL`, `mTLS`, `IP whitelist` и т.д.).

При таком сценарии рекомендуем выносить административный трафик в отдельный Ingress-контроллер (при необходимости с отдельным Ingress-классом) и ограничивать доступ к нему с помощью параметра [`spec.acceptRequestsFrom`](cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom).

При такой конфигурации оба Ingress-ресурса указывают на один и тот же Service:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: admin-ingress
  annotations:
    nginx.ingress.kubernetes.io/whitelist-source-range: "1.2.3.4/32"
spec:
  ingressClassName: nginx
  rules:
    - host: admin.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: public-ingress
spec:
  ingressClassName: nginx
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
```

Приложение при этом может опираться на заголовок `Host` или заголовки `X-Forwarded-*` при принятии решений об авторизации. В такой схеме важно не только настроить правила доступа на уровне Ingress-ресурсов, но и ограничить, с каких адресов можно подключаться к самому Ingress-контроллеру.

Пример Ingress-контроллера, который обслуживает административные Ingress-ресурсы и принимает подключения только из заданных подсетей:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: admin
spec:
  ingressClass: nginx
  inlet: HostPort
  acceptRequestsFrom:
    - 1.2.3.4/32
    - 10.0.0.0/16
  hostPort:
    httpPort: 80
    httpsPort: 443
    behindL7Proxy: true
```

В этом примере:

- Ingress-контроллер доступен на портах узлов через инлет `HostPort`;
- Параметр [`acceptRequestsFrom`](cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) разрешает подключение к контроллеру только из перечисленных подсетей;
- Даже если внешний балансировщик или клиент может передавать свои значения заголовков `X-Forwarded-*`, решение о допуске соединения до контроллера принимается по реальному адресу подключения, а не по заголовкам.
- Административные Ingress-ресурсы (в данном примере `admin-ingress`) обслуживаются этим контроллером согласно настроенному Ingress-классу.

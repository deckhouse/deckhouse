Модуль nginx-ingress
=======

Модуль устанавливает **один или несколько** [nginx-ingress controller'ов](https://github.com/kubernetes/ingress-nginx/) и учитывает все особенности интеграции с кластерами Kubernetes различных типов.


Конфигурация
------------

### Что нужно настраивать?

**Важно!** В абсолютном большинстве случаев **ничего не нужно настраивать**! Лучший конфиг — пустой конфиг.

### Параметры

Модуль поддерживает несколько контроллеров — один **основной** и сколько угодно **дополнительных**, для них можно указывать следующие параметры:
* `inlet` — способа поступления трафика из внешнего мира.
    * Определяется автоматическиьв зависимости от типа кластера!
    * Поддерживаются следующие inlet'ы
        * `LoadBalancer` (автоматически для `GCE` и `ACS`) — заказывает автоматом LoadBalancer.
        * `AWSClassicLoadBalancer` (автоматически для`AWS`) — заказывает автоматом LoadBalancer и включает proxy protocol, используется по-умолчанию для AWS.
        * `Direct` (автоматически `Manual`) — pod'ы работают в host network, nginx слушает на 80 и 443 порту, хитрая схема с direct-fallback.
        * `NodePort` — создает сервис с типом NodePort, подходит в тех ситуациях, когда необходимо настроить "сторонний" балансировщик (например, использовать AWS Application Load Balancer, Qrator или  CloudFLare).
    * Очень наглядно посмотреть отличия четырех типов inlet'ов можно [здесь](modules/nginx-ingress/templates/controller.yaml).
* `config.hsts` — bool, включен ли hsts.
    * По-умолчанию выключен.
* `config.setRealIPFrom` — список CIDR'ов, с которых разрешено использовать заголовок `X-Forwarded-For` в качестве адреса клиента.
    * Список строк, именно YAML list, а не строка со значениями через запятую!
    * Так-как nginx ingress не поддерживает получение адреса клиента из `X-Forwarded-For` при одновременном использовании proxy protocol параметр полностью игнорируется для inlet'а `AWSClassicLoadBalancer`.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role/frontend":""}`.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"node-role/frontend","operator":"Exists"}]`.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
* (только для дополнительных контроллеров) `name` (обязательно) — название контроллера.
    * Используется в качестве суффикса к имени namespace `kube-nginx-ingress-{{ $name }}` и в качестве суффикса к названию класса nginx `nginx-{{ $name }}` (того самого класса, который потом указывается аннотации `kubernetes.io/ingress.class` к ingress ресурсам).


### Пример конфига

```yaml
nginxIngress: |
  config:
    hsts: true
    setRealIPFrom:
    - 4.4.4.4
  nodeSelector: false
  tolerations:
  - key: node-role/frontend
    operator: Exists
  additionalControllers:
  - name: direct
    inlet: Direct
    config:
      hsts: true
      setRealIPFrom:
      - 1.2.3.4/16
      - 4.4.4.4/24
    nodeSelector:
      node-role/direct-frontend: ""
    tolerations:
    - key: node-role/direct-frontend
      operator: Exists
  - name: someproject
    inlet: NodePort
    nodeSelector: false
    tolerations: false
  - name: foo
```

### Особенности использования дополнительных контроллеров

* Для каждого дополнительного контроллера обязательно указывается `name`, при этом разворачивается полная копия всего в отдельном namespace с названием `kube-nginx-ingress-<name>`
* Дополнительные экземпляры контроллера работают с отдельным классом, который необходимо указывать в ingress ресурсах через аннотацию `kubernetes.io/ingress.class: "nginx-<name>"`.

Примеры использования
---------------------

### Bare Metal + Qrator

Кейс:
* Не production площадки (test, stage, etc) и инфраструктурные компоненты (prometheus, dashboard, etc) ходят напрямую.
* Все ресурсы production ходят через Qrator.

Способ реализации:
* Оставляем основной контроллер работать без измений.
* Указываем дополнительный контроллер с inlet `NodePort`.
* В ingress ресурсах прода указываем аннотацию `kubernetes.io/ingress.class: "nginx-qrator"`.
* Настраиваем Qrator, чтобы он отправлял трафик на "эфемерные" порты сервиса с типом NodePort: `kubectl -n kube-nginx-ingress-qraror get svc nginx -o yaml`

```
nginxIngress: |
  additionalControllers:
  - name: qrator
    inlet: NodePort
    config:
      setRealIPFrom:
      - 87.245.197.192
      - 87.245.197.193
      - 87.245.197.194
      - 87.245.197.195
      - 87.245.197.196
      - 83.234.15.112
      - 83.234.15.113
      - 83.234.15.114
      - 83.234.15.115
      - 83.234.15.116
      - 66.110.32.128
      - 66.110.32.129
      - 66.110.32.130
      - 66.110.32.131
      - 130.117.190.16
      - 130.117.190.17
      - 130.117.190.18
      - 130.117.190.19
      - 185.94.108.0/24

```


### AWS + CloudFlare

Кейс:
* Большая часть production ресурсов, все не production ресурсы (test, stage, etc) и инфраструктурные компоненты (prometheus, dashboard, etc) ходят через обычный AWSClassicLoadBalancer.
* Однако часть production ресурсов надо отправить через CloudFront, а setRealIPFrom не поддерживается при использовании AWSClassicLoadBalancer (из-за несовместимости с proxy protocol).

Способ реализации:
* Оставляем основной контроллер работать без измений.
* Указываем дополнительный контроллер с inlet `NodePort`.
* Настраиваем CloudFlare, чтобы он отправлял трафик на адрес сервиса: `kubectl -n kube-nginx-ingress-cf get svc nginx -o yaml`

```
nginxIngress: |
  additionalControllers:
  - name: cf
    inlet: LoadBalancer
    config:
      setRealIPFrom:
      - 103.21.244.0/22
      - 103.22.200.0/22
      - 103.31.4.0/22
      - 104.16.0.0/12
      - 108.162.192.0/18
      - 131.0.72.0/22
      - 141.101.64.0/18
      - 162.158.0.0/15
      - 172.64.0.0/13
      - 173.245.48.0/20
      - 188.114.96.0/20
      - 190.93.240.0/20
      - 197.234.240.0/22
      - 198.41.128.0/17
```

### AWS + AWS Application Load Balancer

Кейс:
* У клиента уже есть сертификаты, заказанные в Amazon и их оттуда никуда не вытащищь.
* Не хочется делать несколько контроллеров и несколько LoadBalancer'ов в Amazon, чтобы сэкономить деньги.

Способ реализации:
* Будем используем в качестве основной и единственной точки входа AWS Application Load Balancer.
* Для этого перенастраиваем основной контроллер с inlet `NodePort`.
* Настраиваем в AWS Application Load Balancer, чтобы он кидал трафик по "эфемерным" портам сервиса с типом NodePort: `kubectl -n kube-nginx-ingress get svc nginx -o yaml`.

```
nginxIngress: |
  inlet: NodePort
  config:
    setRealIPFrom:
    - 0.0.0.0/0
```

### AWS + AWS HTTP Classic Load Balancer

Кейс:
* Все ходит через обычный `AWSClassicLoadBalancer`, но нужно заказать сертификат в Amazon, а его нельзя повесить на существующий AWS Classic Load Balancer.


Способ реализации:
* Оставляем основной контроллер работать без измений.
* Указываем дополнительный контроллер с inlet `NodePort`.
* Создаем (руками или через infra проект в gitlab) сколько необходимо сервисов (со специальными аннотацяими для подключения сертификатов)

```
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-backend-protocol: http
    service.beta.kubernetes.io/aws-load-balancer-ssl-cert: arn:aws:acm:eu-central-1:206112445282:certificate/23341234d-7813-45e8-b249-123421351251234
    service.beta.kubernetes.io/aws-load-balancer-ssl-ports: "443"
  name: nginx-site-1
  namespace: kube-nginx-ingress-aws-http
spec:
  externalTrafficPolicy: Local
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 80
  - name: https
    port: 443
    protocol: TCP
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```


```
nginxIngress: |
  additionalControllers:
  - name: aws-http
    inlet: NodePort
    config:
      setRealIPFrom:
      - 0.0.0.0/0
```

### Bare Metal + несколько проектов, которые не должны быть аффилированны

Кейс:
* Есть основной проект и два дополнительных, но никто не должен знать, что они принадлежат одним владельцам (хостятся в одной площадке).

Способ реализации:
* Выделяем основной контроллер на отдельные машины (ставим на них label и taint `node-role/frontent`)
* Создаем два дополнительных контроллера и выделенные для них машины (с label и taint `node-role/frontend-foo` и `node-role/frontend-bar`)

```
nginxIngress: |
  additionalControllers:
  - name: foo
    nodeSelector:
      node-role/frontend-foo: ""
    tolerations:
    - key: node-role/frontend-foo
      operator: Exists
  - name: bar
    nodeSelector:
      node-role/frontend-bar: ""
    tolerations:
    - key: node-role/frontend-bar
      operator: Exists
```


Дополнительная информация
-------------------------

### Ключевые отличия в работе балансировщиков в разных Cloud

* При создании Service с `spec.type=LoadBalancer` Kubernetes создает сервис с типом `NodePort` и, дополнительно, лезет в клауд и настраивает балансировщик клауда, чтобы он бросал трафик на все узлы Kubernetes на определенные `spec.ports[*].nodePort` (генерятся рандомные в диапазоне `30000-32767`).
* В GCE и Azure балансировщик отправляет трафик на узлы сохраняя source адрес клиента. Если при создании сервиса в Kubernetes указать `spec.externalTrafficPolicy=Local`, то Kubernetes приходящий на узел трафик не будет раскидывать по всем узлам, на которых есть endpoint'ы, а будет кидать только на локальные endpoint'ы, находящиеся на этом узле, а если их нет — соединение не будет устанавливаться. Подробнее об этом [тут](https://kubernetes.io/docs/tutorials/services/source-ip/) и [особенно тут](https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/#preserving-the-client-source-ip).
* В AWS все интересней.
    * До версии Kubernetes 1.9 единственным типом LB, который можно было создать в AWS из Kubernetes, был Classic. При этом по-умолчанию создается AWS Classic LoadBalancer, который проксирует TCP трафик (так же на `spec.ports[*].nodePort`). Трафик при этом приходит не с адреса клиента, а с адресов LoadBalancer'а. И единственный способ узнать адрес клиента — включить proxy protocol (это можно сделать через [аннотацию сервиса в Kubernetes](https://github.com/kubernetes/kubernetes/blob/master/pkg/cloudprovider/providers/aws/aws.go).
    * Начиная с версии Kubernetes 1.9 [можно заводить Network LoadBalancer'ы](https://kubernetes.io/docs/concepts/services-networking/service/#network-load-balancer-support-on-aws-alpha). Такой LoadBalancer работает аналогично Azure и GCE — отправляет трафик с сохранением source адреса клиента.

## Можно ли сделать так, чтобы по умолчанию все было закрыто и нужно было явно разрешать?

### По-умолчанию в сервисам в ns foo кластера F можно подключаться только из других сервисов в этом ns.

Одним глобальным правилом — нет. В каждый NS потребуется стиражировать вот этот ресурсик:
```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-intra-namespace-only
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       namespaces: ["myns"]
```

Можно отрубить другие неймспейсы топором. Этим ресурсиком можно попросить istiod не рассылать конфигурацию, не касающуюся данного NS:
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: Sidecar
metadata:
  name: default
  namespace: d8-istio
spec:
  egress:
  - hosts:
    - "./*"
    - "d8-istio/*"
```
(Если положить это правило в NS d8-istio, то правило станет глобальным).


### Должна быть простая возможность:

#### разрешить ns bar (из нашего кластера)

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-bar
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       namespaces: ["bar"]
```

#### разрешить любым ns из нашего кластера

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-my-cluster
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principal: ["mycluster.local/*"]
```

#### разрешить ns baz из кластера jjjj

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-jjjj-cluster-and-ns-baz
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       namespaces: ["baz"]
       principal: ["jjjj.local/*"]
```

#### разрешить ns baz из кластеров jjjj или ffff

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-jjjj-or-ffff-cluster-to-ns-baz
 namespace: baz
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principal: ["jjjj.local/*", "ffff.local/*"]
```

#### разрешить чему угодно из кластеров aaa, bbb и ddd

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-aaa-or-bbb-or-ddd-clusters
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principal: ["aaa.local/*", "bbb.local/*", "ddd.local/*"]
```

#### разрешить из любого кластера (по mtls)

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-any-cluster-with-mtls
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principal: ["*"] # to set MTLS mandatory
```

#### разрешить вообще откуда угодно (в том числе без mtls)

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-any
 namespace: myns
spec:
 action: ALLOW
 rules: [{}]
```

## Вопросы масштабирования:

### Один кластер:

#### Как влияет количество сервисов в одном кластере на размер envoy’ев в нем?

Больше сервисов –> жирнее envoy. Зависимость линейная. Можно сократить количество сервисов, о которых будет передана информация в envoy с помощью ресурса Sidecar. Например, можно глобально исключить из discovery все сервисы в NS d8-\*.
При этом:
* Request rate doesn’t affect the memory consumption.
* In a large namespace, the proxy consumes approximately 50 MB of memory. (Это если ограничить конфиг этим NS).
* Since the sidecar proxy performs additional work on the data path, it consumes CPU and memory. As of Istio 1.7, a proxy consumes about 0.5 vCPU per 1000 requests per second.

#### Какое предельное количество сервисов в одном кластере?

Ограничений нет.

#### Какое предельное количество подов в одном сервисе?

Ограничений нет.

#### Какое предельное количество подов всего?

Ограничений нет.

#### Если наш сервис A подключается только к одному сервису, у него в памяти будет только то, что надо?

По умолчанию — в памяти будет всё. При желании — можно ограничить с помощью ресурса Sidecar.

#### Если наш сервис A подключается ко всем сервисам в кластере, то у него будут подняты коннекты со всеми подами всех сервисов?

Пока прикладной коннект активен — активен коннект между энвоями. "На всякий случай" энвой ничего не держит.

### Межкластерная фигня:

#### Как влияет количество сервисов в кластере X на размер envoy’ев во всех остальных кластерах? А количество подов?

Линейно. Больше сервисов — больше записей во всех энвоях на всех кластерах.

#### Как влияет количество сервисов которыми пользуется сервис из кластера X в кластере Y на размер gateway?

Sidecar для gateway???

#### Сколько кластеров можно собрать в меш мешей?
#### Вот у нас 50 кластеров.

##### В каждом кластере есть по 3 сервиса, которые пользуются 10 сервисами из разных других кластеров, сколько и где будет открытых tcp сессий и вот этого всего?

* Одно соединение к apiserver.
* Каждый реквест — это новое TCP-соединение. Если реквест с кипэлайвом, то соединение висит пока жив кипэлайв.

##### Тоже самое, но мы практически не подключаемся между кластерами?

* Одно соединение к apiserver.
* Каждый реквест — это новое TCP-соединение. Если реквест с кипэлайвом, то соединение висит пока жив кипэлайв.

### Вопросы безопасности:

#### Доступ istio нужен к своему кластеру rw, а к другим ro?

Да. Для удалённых кластеров создан отдельный ServiceAccount, под который мы генерим kubeconfig.

#### Доступ к другим кластерам – можно же его сильно ограничить (только несколько нужных объектов)?

Можно.

## Вопросы отказоустойчивости:

### Взять и удостовериться, что

#### control-plane можно шатать

* Шатать (рестартить, например) control-plane istio относительно безопасно.

#### gateway’и можно шатать

* Если исчез gateway — начинаются прикладные проблемы, таймауты и пр. Видимо, надо тщательно настраивать таймауты и Circuit-breaker-ы.

### Что будет, если исчез control-plane куба?



### Что будет, если исчез control-plane istio?

* Всё, что работало — продолжает работать. Но:
  * Новые поды не создаются.
  * Новые сервисы не создаются.
* После починки поломанные поды сразу не появляются. Надо как-то бодрить.
* После починки новые сервисы додискавериваются сразу.

### Что будет, если исчез control-plane соседнего кластера (но сервисы отвечают)?

* Все сервисы с соседнего кластера доступны.
* Рестарты всего и вся на локальном кластере ничего не ломают.

### Что будет, если вся эта хуйня хлюпает?

## Вопросы производительности:

### Сам istio

#### Вопросы:

##### Сколько request’ов в секунду?
##### Сколько новых коннекшенов в секунду?
##### Сколько пакетов в секунду?
##### Сколько мегабит в секунду?

#### Где:

##### в sidecar’ах при egress? в sidecar’ах при ingress? (объединил)

* The Envoy proxy uses 0.35 vCPU and 40 MB memory per 1000 requests per second going through the proxy.
* Istiod uses 1 vCPU and 1.5 GB of memory.
* As of Istio 1.7, a proxy consumes about 0.5 vCPU per 1000 requests per second.
* The memory consumption of the proxy depends on the total configuration state the proxy holds. A large number of listeners, clusters, and routes can increase memory usage. (Solution — Sidecar CR).
* Since the proxy normally doesn’t buffer the data passing through, request rate doesn’t affect the memory consumption.
* In the default configuration of Istio 1.8.1 (i.e. Istio with telemetry v2), the two proxies add about 2.65 ms and 2.91 ms to the 90th and 99th percentile latency.


##### в gateway при ingress?

* Память — как у sidecar.
* Латенси должно быть меньше так как нет DNAT.

#### В чем:

##### на одно ядро?
##### на 1ГБ ram?

### Нагрузка на кубернетес


## Как организовать пропихивание учеток и выдачу сертов?

Какая-то внешняя штука, у которой есть большие привелегии на все кластера (и есть их список)? При добавлении нового кластера эта штука:

* выделяет sub CA...
* во всех кластерах создают учетку для этого кластера и настраивает в нем N линков...
* в этом кластере создает N учетов для всех остальных кластеров и настраивает в них…

Предполагается, что будет два больших меша-мешей: prod и stage.

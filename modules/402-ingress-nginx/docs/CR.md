---
title: "Модуль ingress-nginx: Custom Resources"
---

## IngressNginxController

Параметры указываются в поле `spec`.

### Обязательные параметры
* `ingressClass` — имя ingress-класса для обслуживания ingress nginx controller. При помощи данной опции можно создать несколько контроллеров для обслуживания одного ingress-класса.
    * **Важно!** Если указать значение "nginx", то дополнительно будут обрабатываться ingress ресурсы без аннотации `kubernetes.io/ingress.class`.
* `inlet` — способ поступления трафика из внешнего мира.
    * `LoadBalancer` — устанавливается ingress controller и заказывается сервис с типом LoadBalancer.
    * `LoadBalancerWithProxyProtocol` — устанавливается ingress controller и заказывается сервис с типом LoadBalancer. Ingress controller использует proxy-protocol для получения настоящего ip-адреса клиента.
    * `HostPort` — устанавливается ingress controller, который доступен на портах нод через hostPort.
    * `HostPortWithProxyProtocol` — устанавливается ingress controller, который доступен на портах нод через hostPort и использует proxy-protocol для получения настоящего адреса клиента.
        * **Внимание!** При использовании этого inlet вы должны быть уверены, что запросы к ingress'у направляются только от доверенных источников. Одним из способов настройки ограничения может служить опция `acceptRequestsFrom`.

### Необязательные параметры
* `controllerVersion` — версия ingress-nginx контроллера;
    * По умолчанию берется версия из настроек модуля.
    * Доступные варианты: `"0.25"`, `"0.26"`, `"0.33"`.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

* `loadBalancer` — секция настроек для inlet'а `LoadBalancer`:
    * `annotations` — аннотации, которые будут проставлены сервису для гибкой настройки балансировщика.
        * **Внимание!** модуль не учитывает особенности указания аннотаций в различных облаках. Если аннотации для заказа load balancer'а применяются только при создании сервиса, то для обновления подобных параметров вам необходимо будет пересоздать `IngressNginxController` (или создать новый, затем удалив старый).
    * `sourceRanges` — список CIDR, которым разрешен доступ на балансировщик.
        * Облачный провайдер может не поддерживать данную опцию и игнорировать её.
    * `behindL7Proxy` — включает обработку и передачу X-Forwarded-* заголовков.
        * **Внимание!** При использовании этой опции вы должны быть уверены, что запросы к ingress'у направляются только от доверенных источников.
    * `realIPHeader` — заголовок, из которого будет получен настоящий IP-адрес клиента.
        * По умолчанию `X-Forwarded-For`.
        * Опция работает только при включении `behindL7Proxy`.

* `loadBalancerWithProxyProtocol` — секция настроек для inlet'а `LoadBalancerWithProxyProtocol`:
    * `annotations` — аннотации, которые будут проставлены сервису для гибкой настройки балансировщика.
        * **Внимание!** модуль не учитывает особенности указания аннотаций в различных облаках. Если аннотации для заказа load balancer'а применяются только при создании сервиса, то для обновления подобных параметров вам необходимо будет пересоздать `IngressNginxController` (или создать новый, затем удалив старый).
    * `sourceRanges` — список CIDR, которым разрешен доступ на балансировщик.
        * Облачный провайдер может не поддерживать данную опцию и игнорировать её.

* `hostPort` — секция настроек для inlet'а `HostPort`:
    * `httpPort` — порт для небезопасного подключения по HTTP.
        * Если параметр не указан – возможность подключения по HTTP отсутствует.
        * Параметр является обязательным, если не указан `httpsPort`.
    * `httpsPort` — порт для безопасного подключения по HTTPS.
        * Если параметр не указан – возможность подключения по HTTPS отсутствует.
        * Параметр является обязательным, если не указан `httpPort`.
    * `behindL7Proxy` — включает обработку и передачу X-Forwarded-* заголовков.
        * **Внимание!** При использовании этой опции вы должны быть уверены, что запросы к ingress'у направляются только от доверенных источников. Одним из способов настройки ограничения может служить опция `acceptRequestsFrom`.
    * `realIPHeader` — заголовок, из которого будет получен настоящий IP-адрес клиента.
        * По умолчанию `X-Forwarded-For`.
        * Опция работает только при включении `behindL7Proxy`.

* `hostPortWithProxyProtocol` — секция настроек для inlet'а `HostPortWithProxyProtocol`:
    * `httpPort` — порт для небезопасного подключения по HTTP.
        * Если параметр не указан – возможность подключения по HTTP отсутствует.
        * Параметр является обязательным, если не указан `httpsPort`.
    * `httpsPort` — порт для безопасного подключения по HTTPS.
        * Если параметр не указан – возможность подключения по HTTPS отсутствует.
        * Параметр является обязательным, если не указан `httpPort`.

* `acceptRequestsFrom` — список CIDR, которым разрешено подключаться к контроллеру. Вне зависимости от inlet'а всегда проверяется непосредственный адрес (в логах содержится в поле `original_address`), с которого производится подключение (а не "адрес клиента", который может передаваться в некоторых inlet'ах через заголовки или с использованием proxy protocol).
    * Этот параметр реализован при помощи [map module](http://nginx.org/en/docs/http/ngx_http_map_module.html) и если адрес, с которого непосредственно производится подключение, не разрешен – nginx закрывает соединение (при помощи return 444).
    * По умолчанию к контроллеру можно подключаться с любых адресов.
* `resourcesRequests` — настройки максимальных значений cpu и memory, которые может запросить под при выборе ноды (если VPA выключен, максимальные значения становятся желаемыми).
    * `mode` — режим управления реквестами ресурсов:
        * Доступные варианты: `VPA`, `Static`.
        * По умолчанию `VPA`.
    * `vpa` — настройки статического режима управления:
        * `mode` — режим работы VPA.
            * Доступные варианты: `Initial`, `Auto`.
            * По умолчанию `Initial`.
        * `cpu` — настройки для cpu:
            * `max` — максимальное значение, которое может выставить VPA для запроса cpu.
                * По умолчанию `50m`.
            * `min` — минимальное значение, которое может выставить VPA для запроса cpu.
                * По умолчанию `10m`.
        * `memory` — значение для запроса memory.
            * `max` — максимальное значение, которое может выставить VPA для запроса memory.
                * По умолчанию `200Mi`.
            * `min` — минимальное значение, которое может выставить VPA для запроса memory.
                * По умолчанию `50Mi`.
    * `static` — настройки статического режима управления:
        * `cpu` — значение для запроса cpu.
            * По умолчанию `50m`.
        * `memory` — значение для запроса memory.
            * По умолчанию `200Mi`.
* `hsts` — bool, включен ли hsts ([подробнее здесь](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Strict-Transport-Security)).
    * По умолчанию — выключен (`false`).
* `hstsOptions` — параметры HTTP Strict Transport Security:
    * `maxAge` — время в секундах, которое браузер должен помнить, что сайт доступен только с помощью HTTPS.
        * По умолчанию `31536000` секунд (365 дней).
    * `preload` — добавлять ли сайт в список предзагрузки. Эти списки используются современными браузерами и разрешают подключение к вашему сайту только по HTTPS.
        * По умолчанию `false`.
    * `includeSubDomains` — применять ли настройки hsts ко всем саб-доменам сайта.
        * По умолчанию `false`.
* `legacySSL` — bool, включены ли старые версии TLS. Также опция разрешает legacy cipher suites для поддержки старых библиотек и программ: [OWASP Cipher String 'C' ](https://cheatsheetseries.owasp.org/cheatsheets/TLS_Cipher_String_Cheat_Sheet.html). Подробнее [здесь](https://github.com/deckhouse/deckhouse/blob/master/modules/402-ingress-nginx/templates/controller/configmap.yaml).
    * По умолчанию включён только TLSv1.2 и самые новые cipher suites.
* `disableHTTP2` — bool, выключить ли HTTP/2.
    * По умолчанию HTTP/2 включен (`false`).
* `geoIP2` — опции для включения GeoIP2 (работают только для версии контроллера `"0.33"` и выше):
    * `maxmindLicenseKey` — лицензионный ключ для скачивания базы данных GeoIP2. Указание ключа в конфигурации включает скачивание базы GeoIP2 при каждом старте контроллера. Подробнее о получении ключа [читайте здесь](https://blog.maxmind.com/2019/12/18/significant-changes-to-accessing-and-using-geolite2-databases/).
    * `maxmindEditionIDs` — список ревизий баз данных, которые будут скачаны при старте. Чем отличаются, например, GeoIP2-City от GeoLite2-City можно ознакомиться [в этой статье](https://support.maxmind.com/geolite-faq/general/what-is-the-difference-between-geoip2-and-geolite2-databases/).
        * По умолчанию `["GeoLite2-City", "GeoLite2-ASN"]`
        * Доступные варианты:
            * GeoIP2-Anonymous-IP
            * GeoIP2-Country
            * GeoIP2-City
            * GeoIP2-Connection-Type
            * GeoIP2-Domain
            * GeoIP2-ISP
            * GeoIP2-ASN
            * GeoLite2-ASN
            * GeoLite2-Country
            * GeoLite2-City
* `underscoresInHeaders` — bool, разрешены ли нижние подчеркивания в хедерах. Подробнее [здесь](http://nginx.org/en/docs/http/ngx_http_core_module.html#underscores_in_headers). Почему не стоит бездумно включать написано [здесь](https://www.nginx.com/resources/wiki/start/topics/tutorials/config_pitfalls/#missing-disappearing-http-headers).
    * По умолчанию `false`.
* `customErrors` — секция с настройкой кастомизации HTTP ошибок (если секция определена, то все параметры в ней являются обязательными, изменение любого параметра **приводит к перезапуску всех ingress-nginx контроллеров**);
    * `serviceName` — имя сервиса, который будет использоваться, как custom default backend.
    * `namespace` — имя namespace, в котором будет находиться сервис, используемый, как custom default backend.
    * `codes` — список кодов ответа (массив), при которых запрос будет перенаправляться на custom default backend.
* `config` — секция настроек ingress controller, в которую в формате `ключ: значение(строка)` можно записать [любые возможные опции](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/);
    * **Внимание!** Ошибка в указании опций может привести к отказу в работе ingress controller'а.
    * **Внимание!** Не рекомендуется использовать данную опцию, не гарантируется обратная совместимость или работоспособность ingress controller'а с использованием данной опции.
* `additionalHeaders` — дополнительные header'ы, которые будут добавлены к каждому запросу. Указываются в формате `ключ: значение(строка)`.

{% raw %}
### Примеры
#### Общий пример
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  controllerVersion: "0.33"
  hsts: true
  config:
    gzip-level: "4"
    worker-processes: "8"
  additionalHeaders:
    X-Different-Name: "true"
    Host: "$proxy_host"
  acceptRequestsFrom:
  - 1.2.3.4/24
  resourcesRequests:
    mode: VPA
    vpa:
      mode: Auto
      cpu:
        max: 100m
      memory:
        max: 200Mi
```

#### Пример для AWS (Network Load Balancer)
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

#### Пример для GCP
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
```

#### Пример для Openstack
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main-lbwpp
spec:
  inlet: LoadBalancerWithProxyProtocol
  ingressClass: nginx
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
```

{% endraw %}

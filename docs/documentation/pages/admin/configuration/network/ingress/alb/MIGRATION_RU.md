---
title: "Миграция с ingress-nginx на alb"
permalink: ru/admin/configuration/network/ingress/alb/migration.html
description: "Миграция с модуля ingress-nginx на модуль alb в Deckhouse Kubernetes Platform: переход на Gateway API, переключение трафика и откат."
lang: ru
extractedLinksMax: 4
relatedLinks:
  - title: "ALB средствами Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html
  - title: "ALB средствами Ingress NGINX Controller"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html
  - title: "Балансировка входящего трафика"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/
  - title: "Документация модуля alb"
    url: /modules/alb/
---


В руководстве описана миграция с модуля `ingress-nginx` на модуль `alb`. В рамках этой миграции публикация приложений переходит с Ingress API на Gateway API.

Руководство охватывает различия моделей, подготовку инфраструктуры модуля `alb` с ClusterALBInstance или ALBInstance и управляемым Gateway, перевод приложений на Gateway API, миграцию системных интерфейсов Deckhouse Kubernetes Platform (DKP), переключение трафика на модуль `alb` и откат при необходимости.

Порядок работ:

1. [Подготовка инфраструктуры модуля alb](#step-1-preparing-alb-infrastructure).
1. [Миграция интерфейсов DKP](#step-2-migrating-dkp-interfaces) — только если системные интерфейсы сейчас на Ingress и их нужно перевести на Gateway API.
1. [Миграция публикации приложений](#step-3-migrating-application-publishing).
1. [Переключение трафика на модуль alb](#step-4-switching-traffic-to-alb).
1. [Очистка](#step-5-cleanup).

## Причины перехода на Gateway API {#gateway-api-advantages}

Основные причины перехода с Ingress API на Gateway API:

- Активное сопровождение upstream-проекта Ingress NGINX, используемого в DKP, прекращено. Новые функции, исправления и интеграции в нём не ожидаются. Для новых сценариев публикации приложений используйте Gateway API.
- В отличие от Ingress API, Gateway API описывает маршруты отдельными ресурсами по протоколам, явно настраивает точки приёма трафика, контролирует подключение маршрутов и доступ между неймспейсами. Сложные конфигурации можно задавать ресурсами API, а не только аннотациями конкретного контроллера.
- Gateway API разделяет ответственность между ролями. Администраторы кластера и сети управляют инфраструктурой обработки трафика и объектами Gateway через [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance_v1) или [ALBInstance](/modules/alb/cr.html#albinstance-v1alpha1-spec). Администраторы неймспейса настраивают приём трафика через ListenerSet (hostname, TLS, порты). Разработчики приложения задают маршрутизацию с помощью HTTPRoute и других ресурсов маршрутов. Такое разделение позволяет делегировать настройки, проверять конфигурацию и выполнять поэтапную миграцию.

## Сравнение моделей {#model-comparison}

В этом разделе сравнивается, как описывается инфраструктура обработки трафика, как DKP создаёт её и как правила маршрутизации приложений преобразуются в конфигурацию прокси-сервера.

### Ingress API и ingress-nginx

Ниже представлена схема ресурсов и прохождения трафика через модуль `ingress-nginx`.

![Схема ресурсов и прохождения трафика ingress-nginx](../../../../../images/network/ingress/alb/ingress-nginx-scheme.svg)

Обработка HTTP- и HTTPS-трафика через модуль `ingress-nginx` настраивается следующим образом:

1. Администратор кластера создаёт кластерный объект IngressNginxController.
1. В IngressNginxController указывается имя используемого объекта IngressClass. Если имя не задано, используется `nginx`.
1. DKP обрабатывает объект и создаёт необходимую инфраструктуру, включая IngressClass. Несколько объектов IngressNginxController могут использовать один IngressClass.
1. По умолчанию ресурсы DKP публикуются через IngressClass с именем `nginx`. Другой класс можно выбрать в глобальной конфигурации DKP.
1. Администраторы сети или команды разработки создают объекты Ingress, которые явно или неявно выбирают требуемый IngressClass.
1. Итоговая конфигурация nginx объединяет параметры инфраструктуры из IngressNginxController с объектами Ingress, выбранными по IngressClass.

### Gateway API и alb

Ниже представлена схема ресурсов и прохождения трафика через модуль `alb`.

![Схема ресурсов и прохождения трафика Gateway API](../../../../../images/network/ingress/alb/gateway-api-scheme.svg)

Обработка трафика HTTP, HTTPS, gRPC, TLS, TCP и UDP через модуль `alb` настраивается следующим образом:

1. Администратор кластера создаёт кластерный объект ClusterALBInstance.
1. В ClusterALBInstance задаются параметры инфраструктуры и обязательный параметр `gatewayName`, который определяет управляемый объект Gateway.
1. Контроллер модуля `alb` обрабатывает объект и создаёт управляемый Gateway и необходимую инфраструктуру обработки трафика. Несколько объектов ClusterALBInstance могут использовать одинаковое значение `gatewayName` и, следовательно, один объект Gateway.
1. Если настроен объект Gateway DKP по умолчанию, модули DKP создают для него объекты ListenerSet, HTTPRoute и другие ресурсы Gateway API.
1. Администраторы сети или команды разработки создают ресурсы Gateway API и подключают их к управляемому объекту Gateway.
1. Итоговая конфигурация Envoy Proxy объединяет параметры инфраструктуры из ClusterALBInstance с конфигурацией, заданной ресурсами Gateway API, подключёнными к Gateway.

### Ключевые архитектурные различия

В таблице приведены следующие архитектурные различия:

| Параметр | Ingress API и ingress-nginx | Gateway API и alb |
| --- | --- | --- |
| Связи между ресурсами | Инфраструктура и правила маршрутизации косвенно связаны через IngressClass | Ресурсы образуют явный граф с помощью `parentRefs` и других типизированных ссылок. Корневым объектом является Gateway |
| Разделение ответственности | IngressNginxController определяет инфраструктуру, а Ingress объединяет маршрутизацию приложения с настройками, зависящими от контроллера | Администраторы кластера и сети управляют экземплярами и объектами Gateway, администраторы неймспейса — ListenerSet, разработчики приложения — HTTPRoute и другие ресурсы маршрутов |
| Модель конфигурации | Ingress объединяет основную конфигурацию маршрутизации HTTP- и HTTPS-трафика в одном объекте и использует аннотации, зависящие от контроллера | Слушатели, маршруты, бэкенды и политики представлены отдельными ресурсами, формирующими общую конфигурацию |
| Общая конфигурация и точки входа | Несколько объектов IngressNginxController, использующих один IngressClass, применяют общий набор правил Ingress, но предоставляют отдельные точки входа | Несколько объектов ClusterALBInstance с одинаковым значением `gatewayName` предоставляют отдельные точки входа на основе общей конфигурации Gateway |
| Ссылки между неймспейсами | Ingress, объект Service, указанный в качестве бэкенда, и объект Secret с TLS-сертификатом, как правило, находятся в одном неймспейсе | Доступ к ресурсам в других неймспейсах явно разрешается с помощью ReferenceGrant |
| Поддержка протоколов | Ingress предназначен преимущественно для HTTP- и HTTPS-трафика | Для HTTP, gRPC, TCP, TLS и UDP предусмотрены отдельные типы маршрутов |
| Расширение функциональности | Дополнительные параметры обычно задаются аннотациями, зависящими от реализации контроллера | Больше параметров задаётся с помощью структурированных и проверяемых ресурсов API и политик |
| Жизненный цикл и владение | Возможности делегирования конфигурации инфраструктуры и маршрутизации, а также управления их связями ограничены | Инфраструктура Gateway может оставаться неизменной, пока команды разработки независимо создают, изменяют и удаляют маршруты |
| Подключение маршрутов | Выбор IngressClass задаёт общую связь между объектом Ingress и контроллерами | Объекты Gateway и их слушатели явно определяют, какие маршруты могут к ним подключаться |

{% alert level="warning" %}
Несколько объектов ClusterALBInstance могут ссылаться на один Gateway, но параметры уровня Gateway, например `additionalPorts`, могут конфликтовать. Используйте совместимые параметры Gateway во всех экземплярах, связанных с одним объектом Gateway.
{% endalert %}

В соответствии с моделью разрешения конфликтов Gateway API модуль `alb` формирует итоговую конфигурацию по объекту ClusterALBInstance с наиболее ранней датой создания. Конфликтующие параметры более новых объектов игнорируются, а сведения о конфликте отображаются в их статусе.

## Шаг 1. Подготовка инфраструктуры модуля alb {#step-1-preparing-alb-infrastructure}

На этом шаге выберите тип экземпляра модуля `alb` (ClusterALBInstance или ALBInstance), настройте инлет и подготовьте TLS-сертификаты для точки входа Gateway API.

### Выбор ALBInstance или ClusterALBInstance

При выборе типа экземпляра учитывайте следующее:

- Используйте [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance_v1) для общего или платформенного объекта Gateway, публикации системных интерфейсов DKP, а также если требуется инлет `HostPort`.
- Для публикации системных интерфейсов DKP выполните инструкции из раздела [«Публикация служебных доменов»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#публикация-служебных-доменов).
- Используйте [ALBInstance](/modules/alb/cr.html#albinstance-v1alpha1-spec) для объекта Gateway, выделенного приложению или команде и управляемого в пределах соответствующего неймспейса. Для ALBInstance поддерживается только инлет `LoadBalancer`.
- Подробное сравнение приведено в разделе [«ClusterALBInstance и ALBInstance»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#clusteralbinstance-and-albinstance).

### Настройка инлета {#inlet-configuration}

В IngressNginxController тип инлета одновременно определяет способ приёма трафика и дополнительные функции, например Proxy Protocol и сквозную передачу TLS-трафика. В модуле `alb` эти параметры настраиваются раздельно:

- ClusterALBInstance поддерживает инлеты `LoadBalancer` и `HostPort`.
- ALBInstance поддерживает инлет `LoadBalancer`.
- Proxy Protocol включается параметром `spec.useProxyProtocol`. Его можно включать и отключать в существующем экземпляре без перезапуска подов Envoy Proxy или пересоздания экземпляра.
- Сквозная передача TLS-трафика настраивается с помощью TLS-слушателя и объекта TLSRoute, а не отдельного типа инлета.

При выборе инлета для модуля `alb` используйте следующее соответствие:

| Инлет IngressNginxController | Конфигурация модуля alb | Особенности миграции |
| --- | --- | --- |
| `LoadBalancer` | ClusterALBInstance или ALBInstance с `spec.inlet.type: LoadBalancer` | Контроллер создаёт объект Service с типом `LoadBalancer` |
| `LoadBalancerWithProxyProtocol` | Инлет `LoadBalancer` с `spec.useProxyProtocol: true` | Настройте внешний балансировщик для передачи Proxy Protocol. Proxy Protocol и HTTP/3 нельзя включить одновременно |
| `LoadBalancerWithSSLPassthrough` | Инлет `LoadBalancer` с TLS-слушателем и объектом TLSRoute | Сквозная передача TLS-трафика относится к конфигурации маршрутизации Gateway API и не является отдельным вариантом инлета |
| `HostPort` | ClusterALBInstance с `spec.inlet.type: HostPort` | Инлет `HostPort` не поддерживается для ALBInstance |
| `HostPortWithProxyProtocol` | ClusterALBInstance с инлетом `HostPort` и `spec.useProxyProtocol: true` | Proxy Protocol и HTTP/3 нельзя включить одновременно |
| `HostPortWithSSLPassthrough` | ClusterALBInstance с инлетом `HostPort`, TLS-слушателем и объектом TLSRoute | Сквозная передача TLS-трафика настраивается независимо от инлета |
| `HostNetwork` | Прямой аналог отсутствует | Используйте ClusterALBInstance с инлетом `HostPort` на отдельной группе узлов или с другим набором портов хоста. Совместное использование одинаковых портов хоста на одних узлах с `ingress-nginx` недопустимо |
| `HostWithFailover` | Прямой аналог отсутствует | Используйте ClusterALBInstance с инлетом `LoadBalancer` и балансировщиком MetalLB. Выполните настройку по [«Пример для bare metal с балансировщиком MetalLB»](/modules/alb/examples.html#bare-metal-metallb) и проверьте отказоустойчивость балансировщика до переключения пользовательского трафика |

{% alert level="warning" %}
Во время миграции модуль `ingress-nginx` с инлетом `HostNetwork` или `HostPort` и модуль `alb` с инлетом `HostPort` не могут использовать одинаковые порты хоста на одних и тех же узлах. Выберите разные группы узлов или другой набор портов хоста для одного из контроллеров.
{% endalert %}

Связанные параметры инлета переносятся следующим образом:

| IngressNginxController | ClusterALBInstance или ALBInstance |
| --- | --- |
| `spec.loadBalancer.annotations` | `spec.inlet.loadBalancer.serviceAnnotations` |
| `spec.loadBalancer.loadBalancerClass` | `spec.inlet.loadBalancer.loadBalancerClass` |
| `spec.loadBalancer.httpPort`, `httpsPort` | `spec.inlet.loadBalancer.httpPort`, `httpsPort` |
| `spec.loadBalancer.sourceRanges` | `spec.inlet.loadBalancer.loadBalancerSourceRanges` |
| `spec.hostPort.httpPort`, `httpsPort` | `spec.inlet.hostPort.httpPort`, `httpsPort` |
| `spec.acceptRequestsFrom` | `spec.acceptRequestsFrom` |
| `spec.*.behindL7Proxy`, `realIPHeader`, `acceptClientIPHeadersFrom` | `spec.originalIPDetection.realIPHeader`, `setRealIPFrom` |

При переносе параметров учитывайте следующее:

- Перенесите только аннотации объекта Service, поддерживаемые целевой реализацией балансировщика.
- Задайте параметр `spec.inlet.loadBalancer.loadBalancerClass` при создании объекта — после создания его изменить нельзя.
- Оставьте порты HTTP `80` и HTTPS `443` по умолчанию либо задайте для порта значение `0` в модуле `alb`, чтобы отключить соответствующий слушатель по умолчанию.
- Настройте параметры HostPort только для ClusterALBInstance. Укажите хотя бы один порт.
- Перенесите необходимые ограничения доступа по исходным CIDR-диапазонам в `spec.acceptRequestsFrom`.
- Явно укажите CIDR доверенных прокси-серверов. Не разрешайте обработку заголовков с адресом клиента от произвольных источников.
- Проверьте работу `loadBalancerSourceRanges` с целевой реализацией балансировщика. Значение передаётся в объект Service типа `LoadBalancer`. Облачный провайдер может не поддерживать или игнорировать этот параметр.

Тип инлета ClusterALBInstance нельзя изменить после создания, а ALBInstance поддерживает только `LoadBalancer`. Чтобы сменить инлет, создайте новый экземпляр с тем же `gatewayName`, проверьте трафик, переключите на него нагрузку и удалите старый экземпляр.

### TLS и сертификаты

{% alert level="warning" %}
Если `ingress-nginx` и `alb` используются одновременно, а сертификаты выпускаются объектами Issuer или ClusterIssuer с механизмом проверки HTTP-01, используйте отдельные ресурсы Certificate и отдельные объекты Secret для путей Ingress API и Gateway API. Иначе возможны конфликты при выпуске или продлении сертификата.
{% endalert %}

Для [объекта Gateway DKP по умолчанию](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#публикация-служебных-доменов) DKP создаёт отдельный ClusterIssuer с механизмом проверки HTTP-01 Let's Encrypt. Отдельные Certificate и Secret нужны только для объектов Issuer и ClusterIssuer с HTTP-01 и не требуются для объектов, настроенных исключительно на DNS-01.

Инструкции по настройке Issuer или ClusterIssuer с механизмом проверки HTTP-01 через Gateway API приведены в разделе [«Добавление собственного HTTP-01 ClusterIssuer или Issuer для ALB»](/modules/alb/faq.html#custom-http01-clusterissuer-alb).

Чтобы завершить подготовку инфраструктуры, выполните следующие действия:

1. Создайте ClusterALBInstance или ALBInstance с выбранным инлетом и параметрами из таблиц выше.
1. Дождитесь готовности экземпляра и управляемого объекта Gateway.
1. Подготовьте отдельные ресурсы Certificate и объекты Secret для пути Gateway API, если одновременно используются Issuer или ClusterIssuer с механизмом проверки HTTP-01.

## Шаг 2. Миграция интерфейсов DKP {#step-2-migrating-dkp-interfaces}

Если системные интерфейсы DKP публикуются через Ingress и их нужно перевести на Gateway API, выполните инструкции из раздела [«Публикация служебных доменов»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#публикация-служебных-доменов). Не все модули DKP уже публикуют служебные HTTPRoute через Gateway API. После настройки шлюза по умолчанию команда `jq` в том же разделе показывает фактический инвентарь уже опубликованных маршрутов в кластере, а не полный перечень возможностей платформы.

Если миграция интерфейсов DKP не требуется, перейдите к шагу 3.

## Шаг 3. Миграция публикации приложений {#step-3-migrating-application-publishing}

Список ресурсов Gateway API, поддерживаемых модулем `alb`, и сведения о владении управляемыми объектами Gateway приведены в разделе [«Поддерживаемые ресурсы Gateway API»](/modules/alb/).

### Конвертация Ingress в Gateway API

Чтобы преобразовать ресурсы Ingress в ресурсы Gateway API, выполните следующие действия:

1. Убедитесь, что целевой ALBInstance или ClusterALBInstance создан на [шаге 1](#step-1-preparing-alb-infrastructure) и готов.
1. Получите неймспейс и имя управляемого объекта Gateway из статуса экземпляра.
1. Преобразуйте каждый объект Ingress и связанную с ним конфигурацию в соответствующие ресурсы ListenerSet, маршруты и политики для этого Gateway. Встроенный инструмент конвертации даёт черновик, однако не для каждой функции `ingress-nginx` есть прямой аналог в Gateway API.
1. Перед применением проверьте сгенерированные манифесты и при необходимости поправьте их.
1. Примените ресурсы:

   ```shell
   d8 k apply -f gateway-api.yaml
   ```

1. Проверьте статусы Gateway, ListenerSet и маршрутов до переключения трафика.

#### Использование встроенного инструмента ingress2gateway

Контроллер Gateway предоставляет HTTP-эндпоинт, который принимает один объект, объект Kubernetes List или YAML с несколькими документами. Конвертер обрабатывает следующие входные ресурсы:

- Ingress — ресурсы, выбранные для преобразования.
- Service — ресурсы, используемые для определения именованных портов и метаданных объектов Service.
- DexAuthenticator — используется для определения `spec.applicationDomain`, если конфигурация внешней аутентификации `ingress-nginx` содержит переменные nginx, например `$host`.

В качестве входных данных передайте все связанные ресурсы, чтобы конвертер мог сохранить поддерживаемые параметры их конфигурации. По умолчанию эндпоинт отключён и доступен только внутри подов контроллера Gateway.

1. Временно включите параметр [`migrations.ingress2Gateway.enabled`](/modules/alb/configuration.html#parameters-migrations-ingress2gateway-enabled) в конфигурации модуля `alb` и дождитесь перезапуска контроллера Gateway:

   ```shell
   d8 k patch moduleconfig alb --type merge \
     --patch '{"spec":{"settings":{"migrations":{"ingress2Gateway":{"enabled":true}}}}}'
   d8 k -n d8-alb rollout status deployment/gateway-controller
   ```

1. Перенаправьте порт эндпоинта из пода контроллера Gateway:

   ```shell
   d8 k -n d8-alb port-forward deployment/gateway-controller 8082:8082
   ```

1. В другом терминале экспортируйте все распознаваемые типы ресурсов и передайте полученный Kubernetes List непосредственно конвертеру:

   ```shell
   converter_url='http://127.0.0.1:8082/ingress2gateway'
   converter_url="${converter_url}?gateway=<GATEWAY_NAMESPACE>/<GATEWAY_NAME>"
   converter_url="${converter_url}&scope=<SCOPE>&ingress-class=<INGRESS_CLASS>"

   d8 k get ingress,service,dexauthenticator \
     --all-namespaces --output yaml | \
     curl --fail-with-body --silent --show-error --request POST \
       --header 'Content-Type: application/yaml' \
       --data-binary @- \
       --output gateway-api.yaml \
       "$converter_url"
   ```

   В URL имена параметров запроса пишутся строчными буквами: `gateway`, `scope`, `ingress-class`. В примере их значения заданы плейсхолдерами:

   - `<GATEWAY_NAMESPACE>` — неймспейс управляемого объекта Gateway из статуса экземпляра на шаге 1;
   - `<GATEWAY_NAME>` — имя управляемого объекта Gateway из того же статуса.

     Пара `<GATEWAY_NAMESPACE>/<GATEWAY_NAME>` — значение параметра `gateway`. Используйте значение по умолчанию `d8-alb/public-gw` только если оно совпадает с вашим Gateway.

   - `<SCOPE>` — `cluster` для кластерной модели или `namespaced` для Gateway в неймспейсе приложения (как у ALBInstance). Если параметр не указать, используется `cluster`;
   - `<INGRESS_CLASS>` — IngressClass исходных ресурсов Ingress. Если параметр не указать, используется `nginx`.

   {% alert level="info" %}
   Размер тела запроса ограничен 8 МБ. В крупных кластерах экспортируйте только переносимые ресурсы и связанные с ними объекты из нужных неймспейсов.
   {% endalert %}

1. Перед применением проверьте файл `gateway-api.yaml` и диагностические сообщения конвертера, добавленные в виде комментариев YAML, затем примените ресурсы.
1. После завершения преобразования отключите эндпоинт:

   ```shell
   d8 k patch moduleconfig alb --type merge \
     --patch '{"spec":{"settings":{"migrations":{"ingress2Gateway":{"enabled":false}}}}}'
   ```

#### Расширение Gateway API с помощью аннотаций

Спецификация Gateway API не охватывает все зависящие от реализации функции управления трафиком, необходимые для работы DKP. Поэтому модуль `alb` использует аннотации HTTPRoute для параметров, которые пока нельзя задать стандартными полями Gateway API.

По мере развития Gateway API модуль `alb` будет постепенно заменять аннотации соответствующими стандартными ресурсами и полями.

При миграции по возможности используйте стандартные поля Gateway API, а аннотации `ingress-nginx` заменяйте только поддерживаемыми аннотациями `alb` — в том числе после применения `gateway-api.yaml`. Актуальный список приведён в разделе [«Поддерживаемые аннотации HTTPRoute»](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#поддерживаемые-аннотации-httproute).

Утилита `ingress2gateway` формирует черновик ресурсов Gateway API и не переносит все аннотации `ingress-nginx` автоматически. После преобразования сверьте Ingress с таблицей ниже и перенесите нужные параметры вручную.

| Аннотация Ingress (`ingress-nginx`) | Эквивалент в модуле `alb` |
| --- | --- |
| `nginx.ingress.kubernetes.io/whitelist-source-range` | Аннотация HTTPRoute `alb.network.deckhouse.io/whitelist-source-range` |
| `nginx.ingress.kubernetes.io/service-upstream` | Аннотация HTTPRoute `alb.network.deckhouse.io/service-upstream` |
| `nginx.ingress.kubernetes.io/upstream-vhost` | Фильтр `URLRewrite` с `hostname` в HTTPRoute ([публикация с Istio-сайдкаром](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#publishing-with-istio-sidecar)) |
| `nginx.ingress.kubernetes.io/auth-url` / `auth-signin` | Аннотации HTTPRoute `alb.network.deckhouse.io/auth-url` и `alb.network.deckhouse.io/auth-signin` |
| `nginx.ingress.kubernetes.io/auth-type: basic` и секрет | Аннотация HTTPRoute `alb.network.deckhouse.io/basic-auth-secret` |
| `nginx.ingress.kubernetes.io/limit-rps` | Аннотация HTTPRoute `alb.network.deckhouse.io/limit-rps` |
| `nginx.ingress.kubernetes.io/proxy-body-size` | Аннотация HTTPRoute `alb.network.deckhouse.io/buffer-max-request-bytes` (значение в байтах) |
| `nginx.ingress.kubernetes.io/proxy-buffer-size` | Аннотация HTTPRoute `alb.network.deckhouse.io/proxy-buffer-size` |
| `nginx.ingress.kubernetes.io/proxy-read-timeout` / `proxy-send-timeout` | Аннотация HTTPRoute `alb.network.deckhouse.io/idle-timeout` (таймаут неактивности, не полная длительность запроса) |
| `nginx.ingress.kubernetes.io/affinity` / cookie | Аннотация HTTPRoute `alb.network.deckhouse.io/session-affinity` |
| `nginx.ingress.kubernetes.io/rewrite-target` | Аннотация HTTPRoute `alb.network.deckhouse.io/rewrite-target` или стандартные фильтры Gateway API |
| `nginx.ingress.kubernetes.io/configuration-snippet` и прочие сниппеты nginx | Прямого аналога нет; пересмотрите конфигурацию под возможности Gateway API и аннотации `alb` |

Если аннотации Ingress нет в таблице и нет поля Gateway API, поведение после миграции может отличаться или пропасть. Проверьте приложение на пути модуля `alb` до переключения пользовательского трафика.

## Шаг 4. Переключение трафика на модуль alb {#step-4-switching-traffic-to-alb}

Используйте `ingress-nginx` и `alb` одновременно, пока обработка трафика через модуль `alb` не будет проверена и не завершится период, предусмотренный для отката.

Переносите отдельные домены или неймспейсы, если для них можно использовать разные DNS-записи. В остальных случаях переключайте общую внешнюю точку входа, изменяя DNS-записи или пул бэкендов внешнего балансировщика.

На этом шаге:

- протестируйте модуль `alb` без смены DNS через `migrationGateway`, когда нужна проверка без изменения DNS (`ingress-nginx` версии `1.1.0` и новее);
- включите `http01CertificateSolverBridging`, если сертификаты пути Gateway API выпускаются через HTTP-01, пока публичный DNS ещё указывает на Ingress;
- выберите способ переключения по текущей схеме входа — в подразделах ниже: автоматический балансировщик, ручной балансировщик или HostNetwork/HostPort;
- перед окончательным переключением пользовательского трафика выполните [проверку переключения](#validating-the-cutover).

### Тестирование модуля alb через Ingress-контроллер

Параметр [`spec.migrationGateway`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-migrationgateway) ресурса IngressNginxController позволяет тестировать обработку запросов от выбранных IP-адресов без изменения DNS.

Запросы из `sourceCIDRs` продолжают поступать через существующую точку входа Ingress-контроллера, но перенаправляются на внутренний объект Service целевого экземпляра модуля `alb`. Остальные запросы продолжают поступать в бэкенды, указанные в исходных ресурсах Ingress.

{% alert level="info" %}
Параметр доступен в модуле `ingress-nginx` версии `1.1.0` и новее.
{% endalert %}

Несмотря на использование в данном руководстве для миграции на модуль `alb`, параметр [`migrationGateway`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-migrationgateway) не привязан к модулю `alb`.

В `serviceRef` можно указать объект Service, предоставляющий точку входа любой реализации Gateway API и принимающий HTTP- и HTTPS-трафик на заданных портах.

Найдите конфигурационный объект Service модуля `alb`:

```shell
d8 k get service --all-namespaces \
  --selector alb.deckhouse.io/configuration-service
```

Настройте исходный IngressNginxController. Сначала укажите в `sourceCIDRs` небольшое количество адресов тестовых клиентов:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: <CONTROLLER_NAME>
spec:
  # Поля spec, не связанные с migrationGateway (inlet, ingressClass и т.д.), опущены.
  migrationGateway:
    sourceCIDRs:
      - <SOURCE_CIDR>
    serviceRef:
      namespace: <SERVICE_NAMESPACE>
      name: <SERVICE_NAME>
      ports:
        http: <HTTP_PORT>
        https: <HTTPS_PORT>
```

где:

- `<CONTROLLER_NAME>` — имя исходного объекта IngressNginxController;
- `<SOURCE_CIDR>` — CIDR клиентов, чьи запросы во время тестирования перенаправляются на модуль `alb`;
- `<SERVICE_NAMESPACE>` — неймспейс конфигурационного Service модуля `alb`;
- `<SERVICE_NAME>` — имя конфигурационного Service модуля `alb`;
- `<HTTP_PORT>` — HTTP-порт целевого Service;
- `<HTTPS_PORT>` — HTTPS-порт целевого Service.

Для выбранных клиентов `migrationGateway` перенаправляет запросы на порты целевого Service и обходит бэкенды и настройки блоков `location` из исходных ресурсов Ingress. HTTP-запросы идут на указанный HTTP-порт. Для HTTPS nginx терминирует входящее TLS-соединение и устанавливает новое TLS-соединение к указанному HTTPS-порту, используя исходное имя хоста в заголовке Host и для SNI. Поддерживаются HTTP, HTTPS и протоколы с HTTP Upgrade, например WebSocket. gRPC и протоколы, не основанные на HTTP, не поддерживаются.

Параметр действует на все ресурсы Ingress, обслуживаемые контроллером. Перед расширением `sourceCIDRs` проверьте каждое имя хоста, доступное с этих адресов, и убедитесь, что аутентификация, перенаправления, обработка заголовков, GeoIP, соединения WebSocket и другие необходимые политики реализованы в конфигурации модуля `alb`.

Параметр `migrationGateway` не поддерживается с инлетами `HostPortWithSSLPassthrough`, `LoadBalancerWithSSLPassthrough`, `HostWithFailover` и с параметром `enableIstioSidecar`. Целевой Service должен принимать обычный HTTP- и HTTPS-трафик без Proxy Protocol — `migrationGateway` его не передаёт.

Если в целевой конфигурации модуля `alb` нужен Proxy Protocol, на время теста через `migrationGateway` отключите его (`spec.useProxyProtocol: false`). Перед переключением пользовательского трафика снова включите его и проверьте через балансировщик, который передаёт Proxy Protocol.

Если перед Ingress-контроллером используется доверенный L7-балансировщик, проверьте определение исходного IP-адреса клиента, прежде чем использовать `sourceCIDRs` для отбора запросов.

Чтобы немедленно вернуть выбранных клиентов на бэкенды, указанные в исходных ресурсах Ingress, удалите `spec.migrationGateway`:

```shell
d8 k patch ingressnginxcontroller <CONTROLLER_NAME> --type json \
  --patch '[{"op":"remove","path":"/spec/migrationGateway"}]'
```

где `<CONTROLLER_NAME>` — имя исходного объекта IngressNginxController.

### Обеспечение проверки HTTP-01 во время миграции

Параметр [`migrationGateway`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-migrationgateway) перенаправляет выбранные запросы приложений.

Параметр [`migrations.http01CertificateSolverBridging`](/modules/alb/configuration.html#parameters-migrations-http01certificatesolverbridging) обеспечивает прохождение проверочных запросов HTTP-01 от cert-manager для Gateway API через Ingress-контроллер, пока публичные DNS-записи указывают на него.

Если используются объекты Issuer или ClusterIssuer с механизмом проверки HTTP-01, выполните следующие действия:

1. Включите перенаправление проверок до запроса сертификатов для точки входа Gateway API:

   ```shell
   d8 k patch moduleconfig alb --type merge \
     --patch '{"spec":{"settings":{"migrations":{"http01CertificateSolverBridging":{"enabled":true,"ingressClassName":"<INGRESS_CLASS>"}}}}}'
   d8 k -n d8-alb rollout status deployment/gateway-controller
   ```

   где `<INGRESS_CLASS>` — IngressClass, через который в данный момент принимается публичный трафик на порту `80`.

1. Оставьте модуль `ingress-nginx` включённым. Контроллер модуля `alb` создаёт временные ресурсы Ingress для объектов HTTPRoute, создаваемых cert-manager для проверки HTTP-01. Благодаря этому запросы проверки проходят через Ingress-контроллер к соответствующему объекту Service.

1. Выпустите сертификаты для точки входа Gateway API до переключения трафика. Проверьте конфигурацию TLS через `migrationGateway` либо подключитесь непосредственно к IP-адресу точки входа Gateway API, переопределив разрешение имени с помощью параметра curl `--resolve`.

1. После переключения DNS или внешнего балансировщика на точку входа Gateway API проверьте выпуск сертификата непосредственно через Gateway API и отключите перенаправление проверок:

   ```shell
   d8 k patch moduleconfig alb --type merge \
     --patch '{"spec":{"settings":{"migrations":{"http01CertificateSolverBridging":{"enabled":false}}}}}'
   ```

### Выбор способа переключения

Выберите способ переключения в зависимости от того, как сейчас принимается внешний трафик: через автоматически создаваемый балансировщик, через балансировщик под ручным управлением или напрямую через HostNetwork/HostPort.

{% tabs Способ переключения %}
{% tab "DNS / автоматически создаваемый LB" %}

#### Автоматически создаваемый балансировщик нагрузки

Чтобы переключить трафик через DNS, выполните следующие действия:

1. Дождитесь готовности балансировщика модуля `alb` и успешного прохождения проверок состояния.
1. Заранее уменьшите TTL DNS.
1. Измените DNS-записи приложений, заменив адрес Ingress-контроллера адресом точки входа Gateway API.

Если Ingress-контроллер использует объект Service типа `LoadBalancer`, автоматически создаваемый облачным провайдером Kubernetes, исходный и целевой контроллеры обычно получают разные IP-адреса или DNS-имена балансировщиков.

Используйте взвешенные DNS-записи для постепенного переключения только при их поддержке DNS-провайдером.

Балансировщик под управлением облачного провайдера обычно не позволяет постепенно переключать отдельные узлы между двумя независимо управляемыми объектами Service.

Если MetalLB или другая реализация использует фиксированный адрес, этот адрес нельзя одновременно назначить двум объектам Service. Переносите его только в рамках согласованного переключения.

{% endtab %}
{% tab "Балансировщик под ручным управлением" %}

#### Балансировщик нагрузки под ручным управлением

Чтобы переключить трафик через внешний балансировщик, выполните следующие действия:

1. Оставьте публичную DNS-запись без изменений.
1. Добавьте в пул бэкендов внешнего балансировщика узлы модуля `alb` и настроенные для них порты и убедитесь в успешном прохождении проверок состояния.
1. Постепенно переключите трафик с бэкендов Ingress-контроллера.
1. Удаляйте старые бэкенды только после проверки.

Такой способ позволяет переключать отдельные узлы и распределять трафик по весам, если внешний балансировщик поддерживает эти функции.

При настройке бэкендов учитывайте следующее:

- Настройте проверки состояния по пути `/healthz` на HTTP-порту модуля `alb`.
- Сохраните исходный протокол и способ определения IP-адреса клиента: согласуйте настройки Proxy Protocol для пользовательского трафика и проверок состояния либо настройте доверие к заголовкам с адресом клиента, которые передаёт L7-балансировщик.
- Если оба контроллера работают на одних узлах, используйте разные порты хоста либо выберите непересекающиеся группы узлов.

{% endtab %}
{% tab "HostNetwork или HostPort" %}

#### Прямой доступ через HostNetwork или HostPort

Чтобы переключить трафик при прямом доступе через HostNetwork или HostPort, выполните следующие действия:

1. Разверните модуль `alb` на отдельной группе узлов или используйте непересекающиеся порты хоста.
1. Для постепенного переключения добавляйте адреса узлов модуля `alb` в DNS-запись и удаляйте адреса узлов Ingress-контроллера после проверки каждого узла.
1. Учитывайте TTL DNS и кеширование на стороне клиентов: при указании адресов узлов DNS не обеспечивает предсказуемое весовое распределение трафика и плавное завершение активных соединений.

DNS не позволяет выбрать контроллеры, использующие разные порты на одном адресе узла. Если клиенты должны продолжать использовать порты `80` и `443`, используйте разные узлы либо добавьте балансировщик или правило NAT.

Инлет `HostWithFailover` не имеет прямого аналога в модуле `alb`. Если требуется эквивалентная отказоустойчивость, используйте инлет `LoadBalancer` с MetalLB.

{% endtab %}
{% endtabs %}

### Проверка переключения {#validating-the-cutover}

{% alert level="warning" %}
Правила обработки трафика в Ingress-контроллере и модуле `alb` могут различаться, в том числе правила формирования и проверки заголовков, обработка протоколов и набор поддерживаемых функций. Перед переключением пользовательского трафика тщательно протестируйте приложения через модуль `alb` и при необходимости адаптируйте их к особенностям его работы.
{% endalert %}

Перед переключением пользовательского трафика:

- Убедитесь, что ресурсы из шага 3 применены, а в статусе целевого ALBInstance или ClusterALBInstance установлены `ready` и `synced`. Проверьте поля конфликтов.
- Проверьте условия в статусах Gateway, ListenerSet и маршрутов, включая принятие ресурсов, разрешение ссылок и готовность слушателей.
- Для каждого имени хоста и протокола проверьте обработку трафика, подключившись напрямую к адресу точки входа Gateway API или с помощью `migrationGateway`: TLS-сертификаты, перенаправления, аутентификацию, долгоживущие соединения и политики приложений.
- Проверьте сохранение IP-адреса клиента, обработку Proxy Protocol или заголовков с адресом клиента, ограничения доступа по источнику и проверки состояния балансировщика.
- Если для HTTP-01 ещё нужно перенаправление проверок, оставьте `http01CertificateSolverBridging` включённым до переключения DNS или балансировщика.
- Убедитесь, что метрики, журналы, оповещения и дашборды позволяют различать трафик и ошибки обоих контроллеров.
- Сохраняйте ресурсы Ingress, Ingress-контроллер, его внешнюю точку входа и действительные сертификаты в течение всего периода, предусмотренного для отката.
- Выберите способ переключения выше и определите критерии отката.

### Откат

Определите критерии отката до переключения. Если при проверке выявлены ошибки:

1. При тестировании через `migrationGateway` удалите `spec.migrationGateway`.
1. При переключении DNS верните адрес Ingress-контроллера и дождитесь истечения предыдущего TTL DNS.
1. При использовании балансировщика под ручным управлением верните узлы и порты Ingress-контроллера в его пул бэкендов.
1. Если публичный трафик снова поступает через Ingress-контроллер, а объекты Issuer или ClusterIssuer для сертификатов Gateway API используют механизм проверки HTTP-01, повторно включите `http01CertificateSolverBridging`.

Не удаляйте исходные ресурсы Ingress, Ingress-контроллер, балансировщик или DNS-записи, пока возможность отката не проверена и не завершился согласованный период стабилизации.

## Шаг 5. Очистка {#step-5-cleanup}

После завершения периода отката выполните следующие действия:

1. Удалите `migrationGateway`, если он ещё задан.
1. Отключите `http01CertificateSolverBridging` и `ingress2Gateway`, если они ещё включены.
1. Удалите ресурсы Ingress и Ingress-контроллеры, которые больше не обслуживают приложения и системные интерфейсы DKP.
1. Удалите неиспользуемые балансировщики, сертификаты, объекты Secret и DNS-записи.
1. Верните обычные значения TTL DNS, убедившись, что клиенты больше не используют старую точку входа.

{% alert level="info" %}
Часть интерфейсов DKP может по-прежнему публиковаться через Ingress API. Не отключайте модуль `ingress-nginx` и не удаляйте связанные объекты, пока нужные интерфейсы не опубликованы через Gateway API и не проверены. Инструкция находится в разделе [«Публикация служебных доменов»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#публикация-служебных-доменов).
{% endalert %}

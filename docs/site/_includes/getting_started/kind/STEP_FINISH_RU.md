<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-finish.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Все установлено, настроено и работает!

Давайте посмотрим что доступно в модуле [monitoring](/documentation/v1/modules/300-prometheus/):

- **Grafana** доступна по адресу [grafana-127-0-0-1.nip.io](http://grafana-127-0-0-1.nip.io).

  Доступ к Grafana ограничен basic-аутентификацией (больше вариантов аутентификации можно получить, включив модуль [user-auth](/documentation/v1/modules/150-user-authn/):
  - Логин — `admin`;
  - Пароль — сгенерирован автоматически. Узнать его можно в ConfigMap `deckhouse` в секции конфигурации модуля `prometheus`, например, выполнив следующую команду:

    ```bash
    kubectl -n d8-system get cm deckhouse -o jsonpath="{.data.prometheus}" | grep password
    ```

    Пример вывода:

    ```
    $ kubectl -n d8-system get cm deckhouse -o jsonpath="{.data.prometheus}" | grep password 
    password: UJvSB4UYTa3fnDOco6LF
    ```
  
  Изучите дэшборды Grafana, поставляемые с Deckhouse.

- **Prometheus** доступен по адресу: [grafana-127-0-0-1.nip.io/prometheus/](http://grafana-127-0-0-1.nip.io/prometheus/)

</div>

Deckhouse, развернутый в кластере kind, вполне пригоден для ознакомления с остальными возможностями, которые понадобятся для полноценных production-окружений. Далее, информация о таких возможностях Deckhouse.

<section class="cards-blocks">

<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Главное
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Dashboard
</h3>
<div class="cards-item__text" markdown="1">
Включите модуль [dashboard](/documentation/v1/modules/500-dashboard/) и получите доступ к Kubernetes Dashboard по адресу [dashboard-127-0-0-1.nip.io](http://dashboard-127-0-0-1.nip.io/)

Для работы Dashboard необходимо включить модуль [user-authz](/documentation/v1/modules/140-user-authz/).
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Status page
</h3>
<div class="cards-item__text" markdown="1">
Включите модуль [upmeter](/documentation/v1/modules/500-upmeter/) чтобы видеть общий статус Deckhouse по адресу [status-127-0-0-1.nip.io](http://status-127-0-0-1.nip.io), а также чтобы получать данные о доступности компонентов Deckhouse по адресу [upmeter-127-0-0-1.nip.io](http://upmeter-127-0-0-1.nip.io).
</div>
</div>

<div style="width: 30%">&nbsp;</div>

</div>
</div>
</section>

<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Деплой первого приложения
</h2>
<div markdown="1">
Deckhouse делает проще настройку доступа CI/CD-системы в кластер для развертывания вашего приложения, позволяет обеспечить его мониторинг по вашим метрикам.
</div>

<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Настройка CI/CD-системы
</h3>
<div class="cards-item__text" markdown="1">
Включите модуль [user-authz](/documentation/v1/modules/140-user-authz/) и [создайте](/documentation/v1/modules/140-user-authz/usage.html#создание-serviceaccount-для-сервера-и-предоставление-ему-доступа) ServiceAccount, который будет осуществлять деплой в кластер.

Результатом станет `kubeconfig`, который можно использовать во всех системах деплоя в Kubernetes.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Направляем трафик на приложение
</h3>
<div class="cards-item__text" markdown="1">
Ознакомьтесь с возможностями модуля [ingress-nginx](/documentation/v1/modules/140-user-authz/).

Создайте `Service` и `Ingress` для вашего приложения.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Мониторинг приложения
</h3>
<div class="cards-item__text" markdown="1">
Включите модуль [monitoring-custom](/documentation/v1/modules/340-monitoring-custom/) и добавьте аннотации `prometheus.deckhouse.io/custom-target: "my-app"` и `prometheus.deckhouse.io/port: "80"` к созданному
Service.
</div>
</div>

</div>
</div>
</section>

<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Внешняя аутентификация
</h2>
<div markdown="1">
С помощью модуля [user-authn](/documentation/v1/modules/150-user-authn/) Deckhouse поддерживает [множество](/documentation/v1/modules/150-user-authn/usage.html)
механизмов внешней аутентификации.
</div>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Настройка DexProvider
</h3>
<div class="cards-item__text" markdown="1">
Например, для включения аутентификации через GitHub можно включить модуль [user-authn](/documentation/v1/modules/150-user-authn/) и [сконфигурировать](/documentation/v1/modules/150-user-authn/usage.html) объект
`DexProvider`. После создания `DexProvider` при попытке доступа ко всем компонентам Deckhouse (Grafana, Dashboard и
т.д.) потребуется аутентификация через GitHub.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Внешняя аутентификация для любого Ingress
</h3>
<div class="cards-item__text" markdown="1">
Чтобы включить внешнюю аутентификацию для любого Ingress-ресурса, необходимо включить модуль [user-authn](/documentation/v1/modules/150-user-authn/) и создать объект
[DexAuthenticator](/documentation/v1/modules/150-user-authn/cr.html#dexauthenticator).
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Внешняя аутентификация для Kubernetes API
</h3>
<div class="cards-item__text" markdown="1">
Включите модуль [user-authn](/documentation/v1/modules/150-user-authn/), настройте [`publishAPI`](/documentation/v1/modules/150-user-authn/faq.html#как-я-могу-сгенерировать-kubeconfig-для-доступа-к-kubernetes-api), установите локально kubectl и создайте `kubeconfig` для внешнего доступа к API в веб-интерфейсе `kubeconfig.example.com`.
</div>
</div>

</div>
</div>
</section>

<div markdown="1">
## Что дальше?

Подробная информация о системе в целом и по каждому компоненту Deckhouse Platform расположена в
[документации](/documentation/v1/).

По всем возникающим вопросам вы всегда можете связаться с нашим [онлайн-сообществом](/community/about.html#online-community).
</div>

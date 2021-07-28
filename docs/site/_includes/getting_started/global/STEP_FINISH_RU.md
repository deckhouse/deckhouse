<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-finish.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Все установлено, настроено и работает!

Рассмотрим дальнейшие возможности Deckhouse, открывающиеся сразу после установки.

По умолчанию, доступ ко всем компонентам осуществляется через Dex c использованием статического пользователя, созданного в кластере во время установки.
Логин — `admin@example.com`, сгенерированный пароль — `<GENERATED_PASSWORD>`.
</div>

<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Главное
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Актуальная документация для Deckhouse в вашем кластере
</h3>
<div class="cards-item__text" markdown="1">
Внутрикластерная документация актуальна для конкретной версии Deckhouse в вашем кластере: [deckhouse.example.com](https://deckhouse.example.com)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Мониторинг
</h3>
<div class="cards-item__text" markdown="1">
Изучите Grafana дэшборды, поставляемые с Deckhouse: [grafana.example.com](https://grafana.example.com)

Для доступа к Prometheus напрямую: [grafana.example.com/prometheus/](https://grafana.example.com/prometheus/)

[Подробнее](/ru/documentation/v1/modules/300-prometheus/) о модуле `monitoring`.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Dashboard
</h3>
<div class="cards-item__text" markdown="1">
Получите доступ к Kubernetes Dashboard: [dashboard.example.com](https://dashboard.example.com)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Status page
</h3>
<div class="cards-item__text" markdown="1">
Узнайте общий статус Deckhouse и его компонентов: [status.example.com](https://status.example.com)

Контролируйте соблюдение SLA с детализацией по каждому компоненту и временному периоду: [upmeter.example.com](https://upmeter.example.com)
</div>
</div>

</div>
</div>
</section>

<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Деплой первого приложения
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Настройка CI/CD системы
</h3>
<div class="cards-item__text" markdown="1">
[Создайте](/ru/documentation/v1/modules/140-user-authz/usage.html#создание-serviceaccount-и-предоставление-ему-доступа)
и выделите права ServiceAccount'у, который будет осуществлять деплой в кластер.

Результатом станет `kubeconfig`, который можно использовать во всех системах деплоя в Kubernetes.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Направляем трафик на приложение
</h3>
<div class="cards-item__text" markdown="1">
Создайте `Service` и `Ingress` для вашего приложения.

[Подробнее](/ru/documentation/v1/modules/402-ingress-nginx/) о возможностях `ingress-nginx`
модуля.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Мониторинг приложения
</h3>
<div class="cards-item__text" markdown="1">
Добавьте аннотации `prometheus.deckhouse.io/custom-target: "my-app"` и `prometheus.deckhouse.io/port: "80"` к созданному
Service'у.

[Подробнее](/ru/documentation/v1/modules/340-monitoring-custom/) в модуле `monitoring-custom`.
</div>
</div>

</div>
</div>
</section>

{% if page.platform_type == 'cloud' %}
<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Другие возможности
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse" style="width: 100%">
<h3 class="cards-item__title text_h3">
Управление узлами
</h3>
<div class="cards-item__text" markdown="1">
{% if page.platform_type == 'cloud' %}
При создании кластера были созданы разные группы узлов. Чтобы увидеть их в кластере, выполните команду `kubectl get
nodegroups`. Подробнее об этом в
[документации](/ru/documentation/v1/modules/040-node-manager/) по модулю управления узлами.

Чтобы отмасштабировать существующие группы, вам достаточно изменить параметры `minReplicas` и `maxReplicas`. При этом,
если они не равны, — у вас автоматически заработает автоскейлинг.

Чтобы создать новые группы вам понадобится создать новый [InstanceClass](/ru/documentation/v1/modules/030-cloud-provider-{{ page.platform_code | downcase }}/cr.html) и
[NodeGroup](https://early.deckhouse.io/ru/documentation/v1/modules/040-node-manager/cr.html#nodegroup), которая на него
ссылается.
{% else %}
# TODO Bare metal!!!
{% endif %}
</div>
</div>

</div>
</div>
</section>
{% endif %}

<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Внешняя аутентификация
</h2>
<div markdown="1">
Deckhouse поддерживает [множество](/ru/documentation/v1/modules/150-user-authn/usage.html)
механизмов внешней аутентификации.
</div>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Настройка DexProvider
</h3>
<div class="cards-item__text" markdown="1">
Например, для включения аутентификации через GitHub можно
[сконфигурировать](/ru/documentation/v1/modules/150-user-authn/usage.html) объект
`DexProvider`. После создания `DexProvider`, при попытке доступа ко всем компонентам Deckhouse (Grafana, Dashboard и
т.д.) потребуются аутентификации через GitHub
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Внешняя аутентификация для любого Ingress
</h3>
<div class="cards-item__text" markdown="1">
Чтобы включить внешнюю аутентификацию для любого Ingress-ресурса, необходимо создать объект
[DexAuthenticator](/ru/documentation/v1/modules/150-user-authn/cr.html#dexauthenticator).
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Внешняя аутентификация для Kubernetes API
</h3>
<div class="cards-item__text" markdown="1">
Настройте
[`publishAPI`](/ru/documentation/v1/modules/150-user-authn/usage.html#внешний-доступ-к-kubernetes-api)
и создайте `kubeconfig` для внешнего доступа к API в веб интерфейсе `kubeconfig.example.com`.
</div>
</div>

</div>
</div>
</section>

<div markdown="1">
## Что дальше?

Подробная информация о системе в целом и по каждому компоненту Deckhouse Platform расположена в
[документации](/ru/documentation/v1/).

По всем возникающим вопросам связывайтесь с нашим [онлайн-сообществом](/ru/community/about.html#online-community).
</div>

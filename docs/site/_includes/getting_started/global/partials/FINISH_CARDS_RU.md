<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Главное
</h2>
<div class="cards-blocks__cards">

{% if page.platform_code != 'existing' and page.platform_code != 'kind' %}
<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
📚 <span class="cards-item__title-text">Документация</span>
</h3>
<div class="cards-item__text">
<p>Документация по установленной в кластере версии Deckhouse.</p>
<p>Имя веб-сервиса: {% include getting_started/global/partials/dns-template-title.html.liquid name="documentation" %}</p>
</div>
</div>
{% endif %}

{% if page.platform_code != 'kind' %}
<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
📊 <span class="cards-item__title-text">Мониторинг</span>
</h3>
<div class="cards-item__text">
<p>Изучите дэшборды Grafana, поставляемые с Deckhouse.</p>
<p>Имя веб-сервиса: {% include getting_started/global/partials/dns-template-title.html.liquid name="grafana" %}</p>
<p>Для доступа к Prometheus: {% include getting_started/global/partials/dns-template-title.html.liquid name="grafana" path="/prometheus/" onlyPath="true" %}</p>
<p><a href="/documentation/v1/modules/300-prometheus/" target="_blank">Подробнее</a> о модуле <code>monitoring</code>.</p>
</div>
</div>
{% endif %}

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
☸ <span class="cards-item__title-text">Dashboard</span>
</h3>
<div class="cards-item__text">
<p>Получите доступ к Kubernetes Dashboard</p>
<p>Имя веб-сервиса: {% include getting_started/global/partials/dns-template-title.html.liquid name="dashboard" %}</p>
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
👌 <span class="cards-item__title-text">Status page</span>
</h3>
<div class="cards-item__text">
<p>Узнайте общий статус Deckhouse и его компонентов.<br />
Имя веб-сервиса: {% include getting_started/global/partials/dns-template-title.html.liquid name="status" %}</p>

<p>Контролируйте соблюдение SLA с детализацией по каждому компоненту и временному периоду.<br />
Имя веб-сервиса: {% include getting_started/global/partials/dns-template-title.html.liquid name="upmeter" %}</p>
</div>
</div>

{% if page.platform_code != 'kind' %}
<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🏭 <span class="cards-item__title-text">Подготовка к production</span>
</h3>
<div class="cards-item__text" markdown="1">
Подготовьте ваш кластер к приему продуктивного трафика.

Воспользуйтесь нашим [чек-листом](/guides/production.html), чтобы убедиться, что вы ничего не забыли.
</div>
</div>

<div style="width: 30%">&nbsp;</div>
{%- endif %}

{% if page.platform_code == 'kind' %}
<div style="width: 30%">&nbsp;</div>
{%- endif %}
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
⟳ <span class="cards-item__title-text">Настройка CI/CD-системы</span>
</h3>
<div class="cards-item__text" markdown="1">
[Создайте](/documentation/v1/modules/140-user-authz/usage.html#создание-serviceaccount-для-сервера-и-предоставление-ему-доступа) ServiceAccount, который будет осуществлять деплой в кластер, и выделите ему права.

Результатом станет `kubeconfig`, который можно использовать во всех системах деплоя в Kubernetes.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🔀 <span class="cards-item__title-text">Направляем трафик на приложение</span>
</h3>
<div class="cards-item__text" markdown="1">
Создайте `Service` и `Ingress` для вашего приложения.

[Подробнее](/documentation/v1/modules/402-ingress-nginx/) о возможностях `ingress-nginx`
модуля.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🔍 <span class="cards-item__title-text">Мониторинг приложения</span>
</h3>
<div class="cards-item__text" markdown="1">
Добавьте аннотации `prometheus.deckhouse.io/custom-target: "my-app"` и `prometheus.deckhouse.io/port: "80"` к созданному
Service'у.

[Подробнее](/documentation/v1/modules/340-monitoring-custom/) о модуле `monitoring-custom`.
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
⚖ <span class="cards-item__title-text">Управление узлами</span>
</h3>
<div class="cards-item__text" markdown="1">
{% if page.platform_type == 'cloud' %}
При создании кластера были созданы две группы узлов. Чтобы увидеть их в кластере, выполните команду `kubectl get
nodegroups`. Подробнее об этом в [документации](/documentation/v1/modules/040-node-manager/) по модулю управления узлами.

Чтобы отмасштабировать существующие группы, вам достаточно изменить параметры `minPerZone` и `maxPerZone`. При этом,
если они не равны, — у вас автоматически заработает автоскейлинг.

Чтобы создать новые группы вам понадобится создать новый [InstanceClass](/documentation/v1/modules/030-cloud-provider-{{ page.platform_code | regex_replace: "^(openstack)_.+$", "\1" | downcase }}/cr.html) и
[NodeGroup](/documentation/v1/modules/040-node-manager/cr.html#nodegroup), которая на него
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

<div markdown="1">
## Что дальше?

Подробная информация о системе в целом и по каждому компоненту Deckhouse Kubernetes Platform расположена в
[документации](/documentation/v1/).

По всем возникающим вопросам вы всегда можете связаться с нашим [онлайн-сообществом](/community/about.html#online-community).
</div>

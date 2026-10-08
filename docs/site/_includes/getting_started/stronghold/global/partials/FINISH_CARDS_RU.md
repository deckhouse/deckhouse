<section class="cards-blocks">
<div class="cards-blocks__content">
<h2 class="cards-blocks__title text_h2">
Начало работы с кластером
</h2>
<div class="cards-blocks__cards">

{% if page.platform_code != 'existing' and page.platform_code != 'kind' %}
<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
📚 <span class="cards-item__title-text">Документация</span>
</h3>
<div class="cards-item__text">
<p>Документация по установленной в кластере версии Deckhouse Platform.</p>
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
<p>Изучите дашборды Grafana, поставляемые с Deckhouse Platform.</p>
<p>Имя веб-сервиса: {% include getting_started/global/partials/dns-template-title.html.liquid name="grafana" %}</p>
<p>Для доступа к Prometheus: {% include getting_started/global/partials/dns-template-title.html.liquid name="grafana" path="/prometheus/" onlyPath="true" %}</p>
<p>Подробнее — <a href="/modules/prometheus/" target="_blank">в документации модуля <code>prometheus</code></a>.</p>
</div>
</div>
{% endif %}

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
☸ <span class="cards-item__title-text">Kubernetes Dashboard</span>
</h3>
<div class="cards-item__text">
<p>Получите доступ к Kubernetes Dashboard.</p>
<p>Имя веб-сервиса: {% include getting_started/global/partials/dns-template-title.html.liquid name="dashboard" %}</p>
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
👌 <span class="cards-item__title-text">Страница состояния</span>
</h3>
<div class="cards-item__text">
<p>Узнайте общий статус Deckhouse Platform и его компонентов.<br />
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
Подготовьте кластер к приёму трафика.

Воспользуйтесь [чек-листом](/products/kubernetes-platform/documentation/v1/guides/production.html), чтобы ничего не упустить.
</div>
</div>
{%- endif %}
</div>
</div>
</section>

<section class="cards-blocks">
<div class="cards-blocks__content">
<h2 class="cards-blocks__title text_h2">
Развёртывание первого приложения
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
⟳ <span class="cards-item__title-text">Настройка CI/CD-системы</span>
</h3>
<div class="cards-item__text" markdown="1">
[Создайте](/modules/user-authz/usage.html#создание-serviceaccount-для-сервера-и-предоставление-ему-доступа) ServiceAccount, от имени которого приложения будут развёртываться в кластере, и выделите ему права.

Результатом станет `kubeconfig`, который можно использовать в любой системе развёртывания в Kubernetes.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🔀 <span class="cards-item__title-text">Маршрутизация трафика</span>
</h3>
<div class="cards-item__text" markdown="1">
Создайте `Service` и `Ingress` для вашего приложения.

Подробнее — [в документации модуля `ingress-nginx`](/modules/ingress-nginx/).
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🔍 <span class="cards-item__title-text">Мониторинг приложения</span>
</h3>
<div class="cards-item__text" markdown="1">
Добавьте аннотации `prometheus.deckhouse.io/custom-target: "my-app"` и `prometheus.deckhouse.io/port: "80"` к созданному
Service.

Подробнее — [в документации модуля `monitoring-custom`](/modules/monitoring-custom/).
</div>
</div>

</div>
</div>
</section>

{% if page.platform_type == 'cloud' %}
<section class="cards-blocks">
<div class="cards-blocks__content">
<h2 class="cards-blocks__title text_h2">
Другие возможности
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse" style="width: 100%">
<h3 class="cards-item__title text_h3">
⚖ <span class="cards-item__title-text">Управление узлами</span>
</h3>
<div class="cards-item__text" markdown="1">
При создании кластера были созданы две группы узлов. Чтобы увидеть их в кластере, выполните команду `d8 k get nodegroups`. Подробнее — [в документации модуля `node-manager`](/modules/node-manager/).

Чтобы отмасштабировать существующие группы, вам достаточно изменить параметры `minPerZone` и `maxPerZone`. При этом,
если они не равны, — у вас автоматически заработает автоскейлинг.

Чтобы создать новые группы, вам понадобится создать новый [InstanceClass](/modules/cloud-provider-{{ page.platform_code | regex_replace: "^(openstack)_.+$", "\1" | downcase }}/cr.html) и
[NodeGroup](/modules/node-manager/cr.html#nodegroup), которая на него
ссылается.
</div>
</div>

</div>
</div>
</section>
{% endif %}

<div markdown="1">
## Следующие шаги

Подробная информация о системе в целом и по каждому компоненту расположена [в документации Stronghold](/products/stronghold/documentation/admin/overview.html).

С вопросами обращайтесь [в онлайн-сообщество Deckhouse](/community/about.html#online-community).
</div>

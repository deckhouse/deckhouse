<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Главное
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" fill="none" viewBox="0 0 52 52" style="display: inline-block; vertical-align: middle; margin-right: 8px;"><g id="sign"><path id="Vector" fill="#0064FF" d="M27.43 33.1h-7.06V18.9h7.06a7.1 7.1 0 0 1 0 14.2Z"/><g id="Group"><g id="Group_2" fill="#00003C"><path id="Vector_2" d="m14.32 19.73-3.79-3.8L.5 26l10.04 10.07 3.78-3.9-1.94-1.85v-8.64l1.94-1.95Z"/><path id="Vector_3" d="m42.28 15.93-3.8 3.8 1.85 1.95v8.64l-1.84 1.85 3.79 3.9L52.3 26 42.28 15.93Z"/><path id="Vector_4" d="m16.37 10.07 3.79 3.8 1.95-1.95h8.6l1.84 1.95 3.9-3.8L26.4 0 16.37 10.07Z"/><path id="Vector_5" d="m34.5 39.98-1.95-1.85-1.84 1.85h-8.6l-1.95-1.85-1.84 1.85-1.95 1.95L26.41 52l10.03-10.07-1.94-1.95Z"/></g></g></g></svg>
<span class="cards-item__title-text">Веб-интерфейс Deckhouse</span>
</h3>
<div class="cards-item__text">
<p>Попробуйте <a href="/products/kubernetes-platform/documentation/v1/user/web/ui.html" target="_blank">веб-интерфейс</a> управления кластером и его основными компонентами.</p>
<p>Имя веб-сервиса: {% include getting_started/global/partials/dns-template-title.html.liquid name="console" %}</p>
</div>
</div>

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
<p>Изучите дашборды Grafana, поставляемые с Deckhouse.</p>
<p>Имя веб-сервиса: {% include getting_started/global/partials/dns-template-title.html.liquid name="grafana" %}</p>
<p>Для доступа к Prometheus: {% include getting_started/global/partials/dns-template-title.html.liquid name="grafana" path="/prometheus/" onlyPath="true" %}</p>
<p><a href="/modules/prometheus/" target="_blank">Подробнее</a> о модуле <code>monitoring</code>.</p>
</div>
</div>
{% endif %}

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
Подготовьте ваш кластер к приему трафика.

Воспользуйтесь нашим [чек-листом](/products/kubernetes-platform/guides/production.html), чтобы убедиться, что вы ничего не забыли.
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
[Создайте](/modules/user-authz/usage.html#создание-serviceaccount-для-сервера-и-предоставление-ему-доступа) ServiceAccount, который будет осуществлять деплой в кластер, и выделите ему права.

Результатом станет `kubeconfig`, который можно использовать во всех системах деплоя в Kubernetes.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🔀 <span class="cards-item__title-text">Направляем трафик на приложение</span>
</h3>
<div class="cards-item__text" markdown="1">
Создайте `Service` и `Ingress` для вашего приложения.

[Подробнее](/modules/ingress-nginx/) о возможностях `ingress-nginx`
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

[Подробнее](/modules/monitoring-custom/) о модуле `monitoring-custom`.
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
nodegroups`. Подробнее об этом в [документации](/modules/node-manager/) по модулю управления узлами.

Чтобы отмасштабировать существующие группы, вам достаточно изменить параметры `minPerZone` и `maxPerZone`. При этом,
если они не равны, — у вас автоматически заработает автоскейлинг.

Чтобы создать новые группы вам понадобится создать новый [InstanceClass](/modules/cloud-provider-{{ page.platform_code | regex_replace: "^(openstack)_.+$", "\1" | replace: "dvp-provider", "dvp" | downcase }}/cr.html) и
[NodeGroup](/modules/node-manager/cr.html#nodegroup), которая на него
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

Подробная информация о системе в целом и по каждому компоненту Deckhouse Kubernetes Platform расположена в [документации](/products/kubernetes-platform/documentation/v1/).

По всем возникающим вопросам вы всегда можете связаться с нашим [онлайн-сообществом](/community/about.html#online-community).
</div>

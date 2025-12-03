<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Главное
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
⚙️ <span class="cards-item__title-text">Прочие настройки</span>
</h3>
<div class="cards-item__text">
<p>Настройте Ingress-контроллер, DNS и создайте пользователей для доступа к веб-интерфейсам кластера.</p>
<p><a href="/products/virtualization-platform/documentation/admin/install/steps/ingress.html" target="_blank">Прочие настройки →</a></p>
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🖥️ <span class="cards-item__title-text">Начало работы с виртуальными машинами</span>
</h3>
<div class="cards-item__text">
<p>Создавайте виртуальные машины, управляйте дисками, образами и настраивайте сетевые политики.</p>
<p><a href="/products/virtualization-platform/documentation/user/overview.html" target="_blank">Руководство пользователя →</a></p>
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

Воспользуйтесь нашим [чек-листом](/products/virtualization-platform/guides/production.html), чтобы убедиться, что вы ничего не забыли.
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

<div markdown="1">
## Что дальше?

Подробная информация о системе в целом и по каждому компоненту платформы расположена в
[документации](/products/virtualization-platform/documentation/admin/overview.html).

По всем возникающим вопросам вы всегда можете связаться с нашим [онлайн-сообществом](/community/about.html#online-community).
</div>

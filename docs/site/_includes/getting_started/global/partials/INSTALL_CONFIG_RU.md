{% assign revision=include.revision %}

{% if revision == 'ee' %}
{% include getting_started/global/EE_ACCESS_RU.md %}
{% endif %}

Ниже сгенерированы рекомендованные настройки для установки Deckhouse Platform {% if revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}:
- `config.yml` — файл первичной конфигурации кластера. Содержит параметры инсталлятора{% if page.platform_type=='cloud' %}, параметры доступа облачного проавайдера{% endif %} и начальные параметры кластера.
- `resources.yml` — описание ресурсов для создания после установки (настройки узлов и ingress-контроллера).

**Обратите внимание**:
- <span class="mustChange">обязательные</span> для самостоятельного заполнения параметры.
- <span class="mightChange">опциональные</span> параметры.

> Полное описание параметров конфигурации cloud-провайдеров вы можете найти [документации](https://deckhouse.io/ru/documentation/v1/kubernetes.html).
>
> Подробнее о каналах обновления Deckhouse Platform (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).

{% snippetcut name="config.yml" selector="config-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/config.yml.{{ include.layout }}.{{ revision }}.inc" syntax="yaml" %}
{% endsnippetcut %}

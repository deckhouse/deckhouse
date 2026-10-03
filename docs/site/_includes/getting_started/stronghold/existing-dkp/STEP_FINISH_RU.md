<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Всё установлено, настроено и работает

Ниже перечислены возможности Deckhouse Stronghold, доступные сразу после установки.

Откройте в браузере `https://stronghold.<PUBLIC_DOMAIN>` и авторизуйтесь, используя настроенный в кластере метод аутентификации.

{% alert level="info" %}
Замените `<PUBLIC_DOMAIN>` на фактическое доменное имя вашего кластера Deckhouse Platform.
{% endalert %}
</div>

{% include getting_started/stronghold/global/partials/FINISH_CARDS_RU.md %}

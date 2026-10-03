<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Всё установлено и настроено

Ниже перечислены возможности Deckhouse Platform, доступные сразу после установки.

Для доступа к внутрикластерной документации выделен домен `documentation` в соответствии с установленным [шаблоном DNS-имён](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). Например, для шаблона DNS-имён `%s.1.2.3.4.sslip.io` веб-интерфейс документации будет доступен по адресу `https://documentation.1.2.3.4.sslip.io`.

Доступ к документации ограничен аутентификацией (больше вариантов аутентификации можно получить, включив модуль [`user-authn`](/modules/user-authn/)):
</div>

{% include getting_started/global/partials/FINISH_CARDS_RU.md %}

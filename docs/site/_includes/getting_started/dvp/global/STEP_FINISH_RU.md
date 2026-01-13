<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Все установлено, настроено и работает!

Ваш кластер готов к работе! Вы успешно настроили:

- master-узел с развернутой Deckhouse Virtualization Platform;
- worker-узел для запуска рабочих нагрузок;
- NFS-хранилище для данных;
- модуль виртуализации для создания виртуальных машин.

Рассмотрим дальнейшие возможности Deckhouse Virtualization Platform, открывающиеся сразу после установки:
</div>

{% include getting_started/dvp/global/partials/FINISH_CARDS_RU.md %}

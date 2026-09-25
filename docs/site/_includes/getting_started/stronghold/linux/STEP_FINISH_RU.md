<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Все установлено, настроено и работает!

Откройте в браузере `http://127.0.0.1:8200` и авторизуйтесь, используя способ входа «Token» и токен `root`, указанный при запуске.

{% alert level="info" %}
Если вы указали при запуске другое значение параметра `-dev-root-token-id`, используйте его вместо `root`.
{% endalert %}

Теперь можно приступить к знакомству с возможностями Deckhouse Stronghold: создать хранилища секретов, настроить методы аутентификации для пользователей и приложений. Подробная информация о системе в целом и по каждому компоненту расположена в [документации](/products/stronghold/documentation/admin/overview.html).

Чтобы остановить Stronghold, нажмите Ctrl+C в терминале, в котором он запущен.

По всем возникающим вопросам вы всегда можете связаться с нашим [онлайн-сообществом](/community/about.html#online-community).
</div>

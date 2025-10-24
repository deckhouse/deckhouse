<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Все установлено, настроено и работает!

Рассмотрим дальнейшие возможности Deckhouse Stronghold, открывающиеся сразу после установки.

По умолчанию, доступ ко всем компонентам осуществляется через [Dex](https://dexidp.io/) c использованием статического пользователя, созданного в кластере во время установки.

**Сгенерированные** на предыдущих шагах данные пользователя:

- Логин — `admin@deckhouse.io`
- Пароль — `<GENERATED_PASSWORD>` (вы также можете найти его в CustomResource `User` в файле `resource.yml`)

Откройте в браузере `http(s)://stronghold.example.com` и авторизуйтесь через Dex, используя указанные логин и пароль.
</div>

{% include getting_started/stronghold/global/partials/FINISH_CARDS_RU.md %}

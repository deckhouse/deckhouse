<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-finish.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Все установлено, настроено и работает!

Рассмотрим дальнейшие возможности Deckhouse, открывающиеся сразу после установки.

По умолчанию, доступ ко всем компонентам осуществляется через [Dex](https://dexidp.io/) c использованием статического пользователя, созданного в кластере во время установки.

**Сгенерированные** на предыдущих шагах данные пользователя:
- Логин — `admin@deckhouse.io`
- Пароль — `<GENERATED_PASSWORD>` (вы также можете найти его в CustomResource `User` в файле `resource.yml`)

Используйте их для доступа к веб-интерфейсу компонентов Deckhouse.
</div>

{% include getting_started/global/partials/FINISH_CARDS_RU.md %}

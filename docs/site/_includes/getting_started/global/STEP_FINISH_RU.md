<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Все установлено, настроено и работает!

Рассмотрим дальнейшие возможности Deckhouse Kubernetes Platform, открывающиеся сразу после установки.

По умолчанию, доступ ко всем компонентам осуществляется через [Dex](https://dexidp.io/) c использованием статического пользователя, созданного в кластере во время установки.

{% unless page.gs_installer %}
**Сгенерированные** на предыдущих шагах данные пользователя:

- Логин — `admin@deckhouse.io`
- Пароль — `<GENERATED_PASSWORD>` (вы также можете найти его в CustomResource `User` в файле `config.yml`)

Используйте их для доступа к веб-интерфейсу компонентов Deckhouse Kubernetes Platform.
{% endunless %}
</div>

{% if page.gs_installer %}
<div markdown="1">

Откройте веб-интерфейс управления кластером, нажав на кнопку «Подключиться и открыть» в строке с созданным кластером на главном экране.

{% offtopic title="Как выглядит кнопка «Подключиться и открыть»..." %}
<img src="/images/gs/installer/open-console.png" alt="Как выглядит кнопка «Подключиться и открыть»..." style="width: 100%;">
{% endofftopic %}

В этом же окне откроется веб-интерфейс управления установленным кластером DKP.

{% offtopic title="Как выглядит веб-интерфейс..." %}
<img src="/images/gs/installer/console.png" alt="Как выглядит веб-интерфейс..." style="width: 100%;">
{% endofftopic %}

{% if page.platform_type == 'baremetal' %}
Если вы **не настраивали** шаблон DNS-имён и **не создавали** Ingress-контроллер во время установки, выполните следующие шаги:
{% else %}
Выполните следующие шаги:
{% endif %}

1. Установите Ingress-контроллер.  
   Перейдите в раздел «Сеть» → «Балансировка» → «Ingress-контроллеры» и создайте там новый Ingress-контроллер, нажав на кнопку «Добавить» и выбрав пункт «Порт хоста».

   {% offtopic title="Как создать Ingress-контроллер..." %}
   <img src="/images/gs/installer/ingress-create.png" alt="Создание Ingress-контроллера" style="width: 100%;">
   {% endofftopic %}

   Введите название и нажмите кнопку «Создать».  
   Если вам необходимо включить HTTPS-доступ к компонентам кластера, включите его в разделе «Сертификат по умолчанию».

   {% offtopic title="Настройки нового Ingress-контроллера..." %}
   <img src="/images/gs/installer/ingress-settings.png" alt="Настройки нового Ingress-контроллера" style="width: 100%;">
   {% endofftopic %}

2. Настройте шаблон DNS-имён, который будет использоваться для компонентов кластера.  
   _Шаблон DNS-имен используется для настройки Ingress-ресурсов системных приложений. Например, за интерфейсом Grafana закреплено имя `grafana`. Тогда, для шаблона `%s.kube.company.my` Grafana будет доступна по адресу `grafana.kube.company.my`, и т.д._  
   Перейдите в раздел «Deckhouse» → «Глобальные настройки» и введите нужный шаблон в поле «Шаблон DNS-имен».

   {% offtopic title="Настройка шаблона DNS..." %}
   <img src="/images/gs/installer/dns-settings.png" alt="Настройка шаблона DNS" style="width: 100%;">
   {% endofftopic %}

</div>
{% endif %}

{% include getting_started/global/partials/FINISH_CARDS_RU.md %}

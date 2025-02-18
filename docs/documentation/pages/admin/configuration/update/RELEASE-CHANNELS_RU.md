---
title: Каналы обновлений
permalink: ru/admin/configuration/update/release-channels.html
lang: ru
---

{% capture asset_url %}{%- css_asset_tag releases %}[_assets/css/releases.css]{% endcss_asset_tag %}{% endcapture %}
<link rel="stylesheet" type="text/css" href='{{ asset_url | strip_newlines  | true_relative_url }}' />

{%- assign releases = site.data.releases.channels | sort: "stability" -%}

{% alert %}
Информацию о том, какие версии Deckhouse находятся в настоящий момент на каких каналах обновлений, а также о планируемой дате смены версии на канале обновлений смотрите на сайте <a href="https://releases.deckhouse.ru" target="_blank">releases.deckhouse.ru</a>.
{% endalert %}

К кластерам как элементам инфраструктуры обычно предъявляются различные требования.

Например, production-кластер, в отличие от кластера разработки, более требователен к надежности: в нем нежелательно часто обновлять или изменять какие-либо компоненты без особой необходимости, при этом сами компоненты должны быть тщательно протестированы.

Deckhouse использует **пять каналов обновлений**. *Мягко* переключаться между ними можно с помощью модуля [deckhouse](modules/deckhouse/): достаточно указать желаемый канал обновлений в [конфигурации](modules/deckhouse/configuration.html#parameters-releasechannel) модуля.

<div id="releases__stale__block" class="releases__info releases__stale__warning" >
  <strong>Внимание!</strong> В этом кластере не используется какой-либо канал обновлений.
</div>

{%- assign channels_sorted = site.data.releases.channels | sort: "stability" %}
{%- assign channels_sorted_reverse = site.data.releases.channels | sort: "stability" | reverse  %}

<div class="page__container page_releases" markdown="0">
<div class="releases__menu">
{%- for channel in channels_sorted_reverse %}
    <div class="releases__menu-item releases__menu--channel--{{ channel.name }}">
        <div class="releases__menu-item-header">
            <div class="releases__menu-item-title releases__menu--channel--{{ channel.name }}">
                {{ channel.title }}
            </div>
        </div>
        <div class="releases__menu-item-description">
            {{ channel.description[page.lang] }}
        </div>
    </div>
{%- endfor %}
</div>
</div>

Для любого из каналов обновлений можно отключить автоматические обновления (за исключением минорных hotfix’ов) или выбрать удобные временные окна (!!!!!link), в которые будут устанавливаться свежие обновления.

Канал обновлений указывается в модуле Deckhouse в параметре `spec.settings.releaseChannel` – если параметр не указан, то автоматическое обновление отключено. Пример конфигурации с каналом `Stable`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
```

При этом, Deckhouse будет каждую минуту проверять данные о релизе на указанном канале обновлений и при появлении нового релиза, скачает его в кластер и создаст Custom Resource `DeckhouseRelease`, после чего начнется обновление согласно установленным правилам.

## Смена канала обновлений

Deckhouse Kubernetes Platform позволяет мягко переключаться между различными каналами обновлений. Чтобы сменить канал, достаточно указать его новое название в настройках модуля Deckhouse. Однако при этом следует учитывать несколько важных моментов.

### Переключение на более стабильный канал

Например, при переходе с канала `Alpha` на `EarlyAccess` Deckhouse:

1. Скачивает данные о релизах из канала `EarlyAccess`.
1. Сравнивает их с уже существующими в кластере Custom Resource `DeckhouseRelease`.

   - Если в кластере есть **более поздние релизы** (например, `v1.68`) со статусом `Pending` (ещё не применены), они будут **удалены**, поскольку в новом канале таких релизов нет.
   - Если **более поздние релизы** уже перешли в статус `Deployed` (успешно установлены), переход на новый релиз не произойдёт сразу. Deckhouse останется на текущем релизе до тех пор, пока на канале `EarlyAccess` не выйдет **ещё более новая версия**, которую можно будет установить.

### Переключение на менее стабильный канал
Например, при переходе с канала `EarlyAccess` на `Alpha` Deckhouse:

1. Скачивает данные о релизах из канала `Alpha`.
1. Сравнивает их с Custom Resource `DeckhouseRelease`, уже существующими в кластере.
1. Далее платформа выполняет обновление в соответствии с **текущими параметрами обновления** (например, режим обновлений `Auto` или `Manual`, окна обновлений и т.д.).

Таким образом, переключение на более стабильный канал может отложить установку новых релизов до тех пор, пока версии в новом канале не станут более свежими, чем уже имеющиеся в кластере. Переключение же на менее стабильный канал сразу задействует релизы, доступные в более быстром (менее стабильном) канале.

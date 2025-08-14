---
title: Каналы обновлений
permalink: ru/deckhouse-release-channels.html
layout: page
toc: false
lang: ru
---

{%- assign assetHash = 'now' | date: "%Y-%m-%d %H:%M:%S" | sha256 -%}
<link href='assets/css/releases.css?v={{ assetHash }}' rel='stylesheet' type='text/css' crossorigin="anonymous" />

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

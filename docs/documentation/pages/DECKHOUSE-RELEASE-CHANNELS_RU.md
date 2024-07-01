---
title: Каналы обновлений
permalink: ru/deckhouse-release-channels.html
layout: page
toc: false
lang: ru
---

<link rel="stylesheet" type="text/css" href='{{ assets["releases.css"].digest_path }}' />
{%- assign releases = site.data.releases.channels | sort: "stability" -%}

<div class="docs__information warning active">
Информацию о том, какие версии Deckhouse находятся в настоящий момент на каких каналах обновлений, а также о планируемой дате смены версии на канале обновлений смотрите на сайте <a href="https://releases.deckhouse.io" target="_blank">releases.deckhouse.io</a>.
</div>  

К кластерам как элементам инфраструктуры обычно предъявляются различные требования.

Например, production-кластер, в отличие от кластера разработки, более требователен к надежности: в нем нежелательно часто обновлять или изменять какие-либо компоненты без особой необходимости, при этом сами компоненты должны быть тщательно протестированы.

Deckhouse использует **пять каналов обновлений**. *Мягко* переключаться между ними можно с помощью модуля [deckhouse](modules/002-deckhouse/): достаточно указать желаемый канал обновлений в [конфигурации](modules/002-deckhouse/configuration.html#parameters-releasechannel) модуля.

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

---
title: Каналы обновлений
permalink: ru/deckhouse-release-channels.html
layout: page
toc: false
lang: ru
---
{::options parse_block_html="false" /}

<link rel="stylesheet" type="text/css" href='{{ assets["releases.css"].digest_path | true_relative_url }}' />
{%- assign releases = site.data.releases.channels | sort: "stability" -%}
<script type="text/javascript" src='{{ assets["release-info.js"].digest_path | true_relative_url }}'></script>

{%- unless site.mode == "local" %}
<h2 class="releases-page__table--title">Текущие версии Deckhouse</h2>
<div class="releases-page__loadblock progress active">Загрузка данных... <img src="{{ assets["loading.gif"].digest_path | true_relative_url }}" /></div>
<div class="releases-page__loadblock failed">Ошибка загрузки данных.</div>
<div class="releases-page__table--content"></div>
{%- endunless %}

<div class="page__container page_releases">

<div class="releases__info">
<p>К кластерам как элементам инфраструктуры обычно предъявляются различные требования.</p>
<p>Например, production-кластер, в отличие от кластера разработки, имеет более высокие требования к надежности, на нем нежелательно часто обновлять или изменять какие-либо компоненты без особой необходимости, а сами компоненты должны быть максимально протестированы.
</p>
Чтобы покрыть самые частые случаи организации окружений, а также с целью повысить качество самого Deckhouse, мы используем <b>пять каналов обновлений</b>.
</div>

<div id="releases__stale__block" class="releases__info releases__stale__warning" >
  <strong>Внимание!</strong> В этом кластере не используется какой-либо канал обновлений.
</div>

{%- assign channels_sorted = site.data.releases.channels | sort: "stability" %}
{%- assign channels_sorted_reverse = site.data.releases.channels | sort: "stability" | reverse  %}

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
{::options parse_block_html="true" /}

Deckhouse может «мягко» переключаться между каналами обновлений с помощью модуля [deckhouse](modules/002-deckhouse/): достаточно указать желаемый канал обновлений в [конфигурации](modules/002-deckhouse/configuration.html#parameters-releasechannel).

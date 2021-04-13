---
title: Каналы обновлений
permalink: releases.html
layout: sidebar-nosidebar
toc: false

---
{::options parse_block_html="false" /}
{% asset releases.css %}
{%- assign releases = site.data.releases.channels | sort: "stability" -%}

<div class="page__container page_releases">

<div class="releases__info">
<p>К кластерам, как элементам инфраструктуры, как правило предъявляются различные требования.</p>
<p>Продуктивный кластер в отличие от кластера разработки имеет более высокие требования надежности, на нем нежелательно часто обновлять или изменять какие-либо компоненты без особой необходимости, компоненты должны быть максимально протестированы.
</p>
Чтобы покрыть самые частые случаи организации окружений, а также с целью повысить качество самого Deckhouse, мы используем <b>пять каналов обновлений</b>.
</div>

<div id="releases__stale__block" class="releases__info releases__stale__warning" >
В кластере не используется какой-либо канал обновлений.  
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
            {{ channel.description }}
        </div>
    </div>
{%- endfor %}
</div>

</div>

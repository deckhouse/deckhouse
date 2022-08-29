---
title: Release channels
permalink: en/deckhouse-release-channels.html
layout: page
toc: false
---
{::options parse_block_html="false" /}
{% asset releases.css %}
{%- assign releases = site.data.releases.channels | sort: "stability" -%}
<script type="text/javascript" src='{{ assets["release-info.js"].digest_path | true_relative_url }}'></script>

{%- unless site.mode == "local" %}
<h2 class="releases-page__table--title">Current Deckhouse versions</h2>
<div class="releases-page__table--wrap"></div>
{%- endunless%}

<div class="page__container page_releases">

<div class="releases__info">
<p>Clusters, as elements of the infrastructure, usually have different requirements.</p>
<p>A production cluster, unlike a development cluster, has higher reliability requirements. Frequent updates and changes to components are undesirable on a productive cluster. Components should be tested as much as possible.
</p>
We use <b>five release channels</b>.
</div>

<div id="releases__stale__block" class="releases__info releases__stale__warning" >
  <strong>Note!</strong> The cluster does not use any release channel.
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

Deckhouse can "soft" switch between release channels using the [deckhouse](modules/002-deckhouse/) module: it is enough to specify the desired release channel in the [configuration](modules/002-deckhouse/configuration.html#parameters-releasechannel).

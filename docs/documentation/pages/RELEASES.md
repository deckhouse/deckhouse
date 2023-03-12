---
title: Release channels
permalink: en/deckhouse-release-channels.html
layout: page
toc: false
---
<link rel="stylesheet" type="text/css" href='{{ assets["releases.css"].digest_path | true_relative_url }}' />
{%- assign releases = site.data.releases.channels | sort: "stability" -%}

<div class="docs__information warning active">
For information about which versions of Deckhouse are currently on which update channels and the planned date of changing the version on the update channel, go on the <a href="https://flow.deckhouse.io" target="_blank">flow.deckhouse.io</a> website.
</div>  

Clusters, as elements of the infrastructure, usually have different requirements.

A production cluster, unlike a development cluster, has higher reliability requirements. Frequent updates and changes to components are undesirable on a productive cluster. Components should be tested as much as possible.

Deckhouse uses **five release channels**, between which it can "soft" switch using the [deckhouse](modules/002-deckhouse/) module: it is enough to specify the desired release channel in the module [configuration](modules/002-deckhouse/configuration.html#parameters-releasechannel).

<div id="releases__stale__block" class="releases__info releases__stale__warning" >
  <strong>Note!</strong> The cluster does not use any release channel.
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

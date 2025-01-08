---
title: Release channels
permalink: en/deckhouse-release-channels.html
layout: page
toc: false
---

{% capture asset_url %}{%- css_asset_tag releases %}[_assets/css/releases.css]{% endcss_asset_tag %}{% endcapture %}
<link rel="stylesheet" type="text/css" href='{{ asset_url | strip_newlines  | true_relative_url }}' />

{%- assign releases = site.data.releases.channels | sort: "stability" -%}

{% alert %}
For information on which versions of Deckhouse are available on which release channels as well as the planned date of the version update for a particular release channel, visit  <a href="https://releases.deckhouse.io" target="_blank">releases.deckhouse.io</a> website.
{% endalert %}

Clusters, as infrastructure elements, usually have to meet various requirements.

A production cluster, unlike a development one, has higher requirements for reliability. In a production cluster, frequent component updates and changes are undesirable. All the cluster components must be thoroughly tested for stable and reliable operation.

Deckhouse uses **five release channels** which you can *soft-switch* between using the [deckhouse](modules/deckhouse/) module: just specify the desired release channel in the module [configuration](modules/deckhouse/configuration.html#parameters-releasechannel).

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

---
title: Information about used software
description: Information about the software used in the Deckhouse Kubernetes Platform.
permalink: en/reference/oss_info.html
---

<p>This page presents a list of third-party software used in the Deckhouse Kubernetes Platform.</p><br />

<div markdown="0">
    <div class="oss">
        {% assign sorted = site.data.ossinfo | sort_natural: 'name' %}
        {% for item in sorted %}
            <div class="oss__item">
                <div class="oss__item-logo">
                    {% if item.logo %}
                        <a href="{{ item.link }}" target="_blank">
                            <img src="{{ item.logo }}" class="oss__item-logo" />
                        </a>
                    {% endif %}
                </div>
                <a href="{{ item.link }}" target="_blank" class="oss__item-title">
                    {{ item.name }}
                </a>
                <div class="oss__item-description">
                    {{ item.description }}
                </div>
                <div class="oss__item-license">
                    {{ item.license }}
                </div>
            </div>
        {% endfor %}
    </div>
</div>

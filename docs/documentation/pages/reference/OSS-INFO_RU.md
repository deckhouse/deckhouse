---
title: Информация об используемом ПО
permalink: ru/reference/oss_info.html
description: Информация о стороннем ПО, используемом в Deckhouse Kubernetes Platform. 
lang: ru
search: open source software, third party software, software components, OSS information, used software, стороннее ПО, используемое ПО
---

<p>На этой странице представлен список стороннего ПО, используемого в Deckhouse Kubernetes Platform.</p><br />

<div markdown="0">
    <div class="oss">
        {%- assign sorted = site.data.ossinfo | sort_natural: 'name' %}
        {%- for item in sorted %}
          {%- if item.name.size > 0 and item.link.size > 0 and item.description.size > 0 and item.license.size > 0 %}
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
          {%- endif %}  
        {%- endfor %}
    </div>
</div>

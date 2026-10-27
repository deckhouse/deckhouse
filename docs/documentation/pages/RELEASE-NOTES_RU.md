---
title: История изменений Deckhouse Platform
permalink: ru/release-notes.html
description: История изменений Deckhouse Platform.
editionsNotAvailable: [cse]
lang: ru
---

{% alert level="info" %}
{{ site.data.i18n.release_notes.patch_note[page.lang] }}
{% endalert %}

{% capture release_notes %}{% include release-notes/RELEASE-NOTES_RU.md %}{% endcapture %}
{{ release_notes | inject_patch_releases: page.lang }}

{% assign assetHash = site.time | date: "%Y-%m-%d %H:%M:%S" | sha256 %}
<link href='/assets/css/faq.css?v={{ assetHash }}' rel='stylesheet' type='text/css' crossorigin="anonymous" />
<script type="text/javascript" src="/assets/js/faq.js?v={{ assetHash }}"></script>

---
title: "The kube-dns module: configuration"
---

<!-- SCHEMA -->

{% alert %}
Based on RFC 1035, DNS messages carried by UDP are restricted to 512 bytes (not counting the IP or UDP headers). Longer messages are truncated and the TC bit is set in the header.
Based on RFC 5966, when the dns-client receives truncated response, it takes the TC flag as an indication that it should retry over TCP instead.
{% endalert %}

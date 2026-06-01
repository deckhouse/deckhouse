<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Everything is installed, configured, and working!

Let's explore the additional capabilities of Deckhouse Stronghold that become available immediately after installation.

Open `https://stronghold.<example.com>` in your browser and log in using the authentication method configured in your cluster.

{% alert level="info" %}
Replace `<example.com>` with the actual domain name of your Deckhouse Kubernetes Platform cluster.
{% endalert %}
</div>

{% include getting_started/stronghold/global/partials/FINISH_CARDS_RU.md %}

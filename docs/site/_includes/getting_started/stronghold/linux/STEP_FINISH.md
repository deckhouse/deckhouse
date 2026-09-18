<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Everything is installed, configured, and working!

Open `http://127.0.0.1:8200` in your browser and log in using the "Token" method and the `root` token specified at startup.

{% alert level="info" %}
If you specified a different value for the `-dev-root-token-id` parameter at startup, use it instead of `root`.
{% endalert %}

Now you can start exploring the capabilities of Deckhouse Stronghold: create secret stores and configure authentication methods for users and applications. Detailed information about the system as a whole and about each component is available in the [documentation](/products/stronghold/documentation/admin/overview.html).

To stop Stronghold, press Ctrl+C in the terminal where it is running.

If you have any questions, feel free to contact our [online community](/community/about.html#online-community).
</div>

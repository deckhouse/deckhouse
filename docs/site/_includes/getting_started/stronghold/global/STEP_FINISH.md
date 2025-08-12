<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Everything is installed, configured, and working!

Now that you have installed and properly configured Deckhouse Stronghold, let's look at what you can do with it.

By default, the [Dex](https://dexidp.io/) is used for accessing all the components.

Here are credentials **generated** in the previous steps:

- Username — `admin@deckhouse.io`
- Password — `<GENERATED_PASSWORD>` (you can also find it in the `User` CustomResource in the `resource.yml` file)

Use them to access the web interface of the components.
</div>

{% include getting_started/stronghold/global/partials/FINISH_CARDS.md %}

<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Everything is installed, configured, and working

Below are the Deckhouse Stronghold capabilities available right after installation.

By default, all components are accessed through [Dex](https://dexidp.io/) using the static user created in the cluster during installation.

Here are credentials **generated** in the previous steps:

- Username — `admin@deckhouse.io`
- Password — `<GENERATED_PASSWORD>` (you can also find it in the User in the `{% if page.platform_type == 'baremetal' %}user.yml{% else %}config.yml{% endif %}` file)

Open `https://stronghold.example.com` in your browser and sign in via Dex using the provided username and password.
</div>

{% include getting_started/stronghold/global/partials/FINISH_CARDS.md %}

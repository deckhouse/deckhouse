<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Everything is installed and configured

The following Deckhouse Platform capabilities are available right after the installation.

For access to the in-cluster documentation the `documentation` domain is reserved in accordance with the [DNS names template](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). E.g., for the DNS names template `%s.1.2.3.4.sslip.io`, the documentation web interface will be available at `https://documentation.1.2.3.4.sslip.io`.

Access to the web interfaces is restricted via the authentication mechanism (additional authentication options are provided in the [`user-authn`](/modules/user-authn/) module).
</div>

{% include getting_started/global/partials/FINISH_CARDS.md %}

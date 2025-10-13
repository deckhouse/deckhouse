<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Everything is installed, configured, and working!

Now that you have installed and properly configured Deckhouse Kubernetes Platform, let's look at what you can do with it.

For access to the in-cluster documentation the `deckhouse` domain is reserved in accordance with the [DNS names template](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). E.g., for the DNS names template `%s.1.2.3.4.sslip.io`, the documentation web interface will be available at `https://deckhouse.1.2.3.4.sslip.io`.

Access to the documentation is restricted via the authentication mechanism (additional authentication options are provided in the [`user-auth`](/modules/user-authn/) module):

- Login — `admin`
- Password was generated automatically. You can find it out by running the command:
  
  - For Deckhouse 1.46 or newer:

    ```bash
    kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- sh -c "deckhouse-controller module values documentation -o json | jq -r '.internal.auth.password'"
    ```

  - For Deckhouse 1.45 or older:

    ```bash
    kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- sh -c "deckhouse-controller module values deckhouse-web -o json | jq -r '.deckhouseWeb.internal.auth.password'"
    ```

  {% offtopic title="Sample output..." %}
```
$ kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- sh -c "deckhouse-controller module values documentation -o json | jq -r '.internal.auth.password'" 
3aE7nY1VlfiYCH4GFIqA
```
  {% endofftopic %}
</div>

{% include getting_started/global/partials/FINISH_CARDS.md %}

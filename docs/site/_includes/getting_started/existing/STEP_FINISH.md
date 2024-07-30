<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-finish.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Congratulations, your Deckhouse Kubernetes platform is up and running!

Now that you have installed and properly configured Deckhouse, let's look at what you can do with it.

For access to the in-cluster documentation the `deckhouse` domain is reserved in accordance with the [DNS names template](../../documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate). E.g., for the DNS names template `%s.1.2.3.4.sslip.io`, the documentation web interface will be available at `https://deckhouse.1.2.3.4.sslip.io`.

Access to the documentation is restricted via the basic authentication mechanism (additional authentication options are provided in the [user-auth](../../documentation/v1/modules/150-user-authn/) module):
- Login — `admin`
- Password was generated automatically. You can find it out by running the command:
  
  - For Deckhouse 1.46 or newer:

    {% snippetcut %}
```bash
kubectl -n d8-system exec deploy/deckhouse -- sh -c "deckhouse-controller module values documentation -o json | jq -r '.documentation.internal.auth.password'"
```
{% endsnippetcut %}

  - For Deckhouse 1.45 or older:

    {% snippetcut %}
```bash
kubectl -n d8-system exec deploy/deckhouse -- sh -c "deckhouse-controller module values deckhouse-web -o json | jq -r '.deckhouseWeb.internal.auth.password'"
```
{% endsnippetcut %}

  {% offtopic title="Sample output..." %}
```
$ kubectl -n d8-system exec deploy/deckhouse -- sh -c "deckhouse-controller module values documentation -o json | jq -r '.documentation.internal.auth.password'" 
3aE7nY1VlfiYCH4GFIqA
```
  {% endofftopic %}
</div>

{% include getting_started/global/partials/FINISH_CARDS.md %}

<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-finish.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Congratulations, your Deckhouse Kubernetes Platform is up and running!

Now that you have installed and properly configured Deckhouse, let's look at what you can do with it.

By default, the [Dex](https://dexidp.io/) is used for accessing all the components.

Here are credentials **generated** in the previous steps:
- Username — `admin@deckhouse.io`
- Password — `<GENERATED_PASSWORD>` (you can also find it in the `User` CustomResource in the `resource.yml` file)

Use them to access the web interface of the Deckhouse components.
</div>

{% include getting_started/global/partials/FINISH_CARDS.md %}

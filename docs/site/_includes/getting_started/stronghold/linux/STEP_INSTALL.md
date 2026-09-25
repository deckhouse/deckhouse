This installation method is intended for a local introduction to Deckhouse Stronghold: it runs in development (dev) mode, keeps data in memory, and requires no Kubernetes cluster.

{% alert level="warning" %}
The development mode is intended for evaluation and testing only. Do not use it in a production environment: all data is lost when the process stops.
{% endalert %}

## Downloading the archive

Downloading the Stronghold archive for Linux OS requires a Deckhouse Stronghold license key for the Enterprise Edition or Certified Security Edition. Select the edition, enter the key, pick a version, and click "Download": the browser saves the `stronghold-<version>.tar` archive with the Linux (amd64) binary.

{% include getting_started/stronghold/linux/partials/download.html.liquid %}

Unpack the downloaded archive, specifying its actual name (it contains the Stronghold version):

<div id="stronghold-unpack" markdown="1">
```bash
tar -xf stronghold-v1.19.3.tar
```
</div>

## Running Stronghold

Run Stronghold in development mode, specifying the root token:

```bash
./stronghold server -dev -dev-root-token-id root
```

Stronghold will be available at `http://127.0.0.1:8200`, and the process stays running in the current terminal.

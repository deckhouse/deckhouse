This installation method is intended for a local introduction to Deckhouse Stronghold: it runs in development (dev) mode, keeps data in memory, and requires no Kubernetes cluster.

{% alert level="warning" %}
The development mode is intended for evaluation and testing only. Do not use it in a production environment: all data is lost when the process stops.
{% endalert %}

## Downloading the archive

Go to the <a href="/products/stronghold/get/" target="_blank">download page</a> and download the Stronghold archive for Linux OS. A license key is required for downloading.

Unpack the downloaded archive, specifying its actual name (it contains the Stronghold version):

```bash
tar -xf stronghold-v1.19.3.tar
```

## Running Stronghold

Run Stronghold in development mode, specifying the root token:

```bash
./stronghold server -dev -dev-root-token-id root
```

Stronghold will be available at `http://127.0.0.1:8200`, and the process stays running in the current terminal.

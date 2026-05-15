<script type="text/javascript" src='{% javascript_asset_tag getting-started-config-highlight %}[_assets/js/getting-started-config-highlight.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag dvp-getting-started-shared %}[_assets/js/dvp/getting-started-dvp-shared.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag dvp-getting-started-access %}[_assets/js/dvp/getting-started-dvp-access.js]{% endjavascript_asset_tag %}'></script>

## Platform configuration

Cluster, NFS, Ingress, user, and project settings are defined in `config.yml` at the installation step. Below is post-bootstrap verification.

CE vs EE: [Kubernetes Platform features](https://deckhouse.io/products/kubernetes-platform/features/), [Virtualization Platform editions](https://deckhouse.io/products/virtualization-platform/documentation/about/editions.html).

### Ingress NGINX

Ensure Ingress controller pods are `Running` (on the master node):

```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
```

### DNS

Create DNS records for the domain template you set on the cluster parameters step (`publicDomainTemplate`). For a wildcard template, a single A record to the node public IP may be enough; otherwise add records for the required subdomains (for example `console`, `grafana`, `prometheus` instead of `%s` in the template).

### Nodes

```bash
sudo -i d8 k get no
```

All nodes should be `Ready`.

### NFS (`csi-nfs`)

```bash
sudo -i d8 k get module csi-nfs -w
sudo -i d8 k get nfsstorageclass
sudo -i d8 k get storageclass
```

Confirm the default StorageClass name matches the NFS StorageClass from the cluster parameters step and `global.defaultClusterStorageClass` in `ModuleConfig/global` (if you kept the install defaults, that is `nfs-storage-class`).

### `virtualization` module

```bash
sudo -i d8 k get po -n d8-virtualization
```

Wait until module pods are `Running`.

### Console access

Open the web UI at `console.<your_domain_suffix>` (replace `%s` in `publicDomainTemplate` with the chosen prefix) and sign in with the administrator account from the cluster parameters step.

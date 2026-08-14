# CNCF Kubernetes conformance (Sonobuoy)

This directory stores **certified conformance** artifacts per Kubernetes minor version: `e2e.log` and `junit_01.xml` produced by [Sonobuoy](https://github.com/vmware-tanzu/sonobuoy) in `certified-conformance` mode.

Use it when you need to refresh bundled results after upgrading or validating the supported Kubernetes line.

---

## 1. Cluster and Deckhouse

Deploy a cluster with Deckhouse at the target Kubernetes version and ensure `kubectl` points at it.

---

## 2. Adjust RBAC and admission (required for a clean run)

Pod Security Standards can also block workloads that the e2e suite expects. For the run, relax **Admission Policy Engine** defaults so workloads are not constrained below what the suite needs.

Apply once (review before production clusters):

```bash
kubectl apply -f - <<'EOF'
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: admission-policy-engine
spec:
  enabled: true
  settings:
    podSecurityStandards:
      defaultPolicy: Privileged
  version: 1
EOF
```

---

## 3. Install Sonobuoy CLI

Download a release from [Sonobuoy releases](https://github.com/vmware-tanzu/sonobuoy/releases), unpack the binary, and put it on your `PATH` (or run from the unpack directory).

Example for Linux `amd64` (replace the version with the one you want):

```bash
curl -sL -o sonobuoy.tgz \
  'https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.57.3/sonobuoy_0.57.3_linux_amd64.tar.gz'
tar -xzf sonobuoy.tgz sonobuoy
chmod +x sonobuoy
```

---

## 4. Run conformance

```bash
./sonobuoy run --mode=certified-conformance --kubeconfig=/etc/kubernetes/super-admin.conf
```

Wait until `./sonobuoy status` reports the run as **completed**. To see progress, check that namespaces are being created and deleted (tests run in isolated namespaces).

The run typically takes **2 hours**.

---

## 5. Fetch only `e2e.log` and `junit_01.xml`

From the machine where the CLI runs:

```bash
./sonobuoy retrieve . -f sb.tar.gz
tar -xzf sb.tar.gz \
  plugins/e2e/results/global/e2e.log \
  plugins/e2e/results/global/junit_01.xml
```

You will get:

`plugins/e2e/results/global/e2e.log`  
`plugins/e2e/results/global/junit_01.xml`

Optional: remove the tarball when done (`rm -f sb.tar.gz`).
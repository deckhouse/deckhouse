# CNCF Kubernetes conformance (Sonobuoy)

This directory stores **certified conformance** artifacts per Kubernetes minor version: `e2e.log` and `junit_01.xml` produced by [Sonobuoy](https://github.com/vmware-tanzu/sonobuoy) in `certified-conformance` mode.

Use it when you need to refresh bundled results after upgrading or validating the supported Kubernetes line.

---

## 1. Cluster and Deckhouse

Deploy a cluster with Deckhouse at the target Kubernetes version and ensure `kubectl` points at it.

---

## 2. Adjust RBAC and admission (required for a clean run)

Conformance runs hit node subresources (for example `/metrics` on kubelet via the apiserver). Without an explicit binding, checks that rely on that path can fail with **403** for identity `kube-apiserver-kubelet-client`.

Pod Security Standards can also block workloads that the e2e suite expects. For the run, relax **Admission Policy Engine** defaults so workloads are not constrained below what the suite needs.

Apply once (review before production clusters):

```bash
kubectl apply -f - <<'EOF'
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:kubelet-api-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:kubelet-api-admin
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: kube-apiserver-kubelet-client
---
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
./sonobuoy run --mode=certified-conformance
```

Wait until `./sonobuoy status` reports the run as **completed**.

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

---

## 6. Add files to this module

Place both files under the minor version directory that matches your cluster, for example:

```text
modules/000-common/images/kubernetes/conformance/<version>/e2e.log
modules/000-common/images/kubernetes/conformance/<version>/junit_01.xml
```

Example: for Kubernetes **1.33** → use directory `1.33/`.

Commit the changes and open a PR.

---

## 7. PR labels (automation)

After the workflow that validates conformance results is present on the **default branch**, add label:

`tests/conformance/<version>`

(for example `tests/conformance/1.33`). Automation reads `junit_01.xml` from this PR branch and, depending on the suite outcome, sets follow-up labels (`passed` / `failed`) and removes the trigger label so you can run the check again.

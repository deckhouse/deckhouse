---
title: "The deckhouse module: FAQ"
---

## How to run kube-bench in my cluster?

First you have to exec in Deckhouse pod
```
kubectl -n d8-system exec -ti deploy/deckhouse -- bash
```

Then you have to select which node you want to run kube-bench

* Run on random node
```
curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl create -f -
```

* Run on specific node, e.g. control-plane node
```
curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | yq r - -j | jq '.spec.template.spec.tolerations=[{"operator": "Exists"}] | .spec.template.spec.nodeSelector={"node-role.kubernetes.io/control-plane": ""}' | kubectl create -f -
```

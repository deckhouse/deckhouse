#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Check force annotation

kubectl patch mc deckhouse --type=merge -p '{"spec": {"settings": {"releaseChannel": "Alpha", "update": {"mode": "Auto", "windows": [{"from": "04:00", "to": "04:01"}]}}}}'

kubectl apply -f - <<"EOF"
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  annotations:
    dryrun: "true"
    release.deckhouse.io/current-restored: "true"
  name: v1.65.0
spec:
  version: v1.65.0
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Deployed deckhouserelease/v1.65.0

kubectl apply -f - <<"EOF"
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  annotations:
    dryrun: "true"
  name: v1.67.0
spec:
  version: v1.67.0
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Pending deckhouserelease/v1.67.0
kubectl wait --for=jsonpath='{.status.message}'="minor version is greater than deployed v1.65.0 by one" deckhouserelease/v1.67.0

kubectl annotate deckhouserelease v1.67.0 release.deckhouse.io/force=true

kubectl wait --for=jsonpath='{.status.phase}'=Deployed deckhouserelease/v1.67.0


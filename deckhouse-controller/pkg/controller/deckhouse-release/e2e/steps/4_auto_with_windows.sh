#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Test Auto update mode with windows set
echo "Run checks in Auto update mode with windows"

kubectl patch mc deckhouse --type=merge -p '{"spec": {"settings": {"releaseChannel": "Alpha", "update": {"mode": "Auto", "windows":[{"from": "03:33", "to": "03:45"}]}}}}'

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
  name: v1.66.0
spec:
  version: v1.66.0
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Pending deckhouserelease/v1.66.0


kubectl patch mc deckhouse --type=merge -p '{"spec": {"settings": {"releaseChannel": "Alpha", "update": {"mode": "Auto", "windows":[{"from": "00:00", "to": "23:59"}]}}}}'
kubectl wait --for=jsonpath='{.status.phase}'=Deployed deckhouserelease/v1.66.0


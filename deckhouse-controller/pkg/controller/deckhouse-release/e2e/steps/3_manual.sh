#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Test Manual update mode
echo "Run checks in Manual update mode"

kubectl patch mc deckhouse --type=merge -p '{"spec": {"settings": {"releaseChannel": "Alpha", "update": {"mode": "Manual"}}}}'

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
  name: v1.65.5
spec:
  version: v1.65.5
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Pending deckhouserelease/v1.65.5
kubectl annotate deckhouserelease v1.65.5 release.deckhouse.io/approved=true
kubectl wait --for=jsonpath='{.status.phase}'=Deployed deckhouserelease/v1.65.5

kubectl apply -f - <<"EOF"
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  annotations:
    dryrun: "true"
  name: v1.65.6
spec:
  version: v1.65.6
EOF

kubectl apply -f - <<"EOF"
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  annotations:
    dryrun: "true"
  name: v1.65.7
spec:
  version: v1.65.8
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Skipped deckhouserelease/v1.65.6
kubectl wait --for=jsonpath='{.status.phase}'=Pending deckhouserelease/v1.65.7

kubectl annotate deckhouserelease v1.65.7 release.deckhouse.io/approved=true
kubectl wait --for=jsonpath='{.status.phase}'=Deployed deckhouserelease/v1.65.7


kubectl apply -f - <<"EOF"
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  annotations:
    dryrun: "true"
  name: v1.66.2
spec:
  version: v1.66.2
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Pending deckhouserelease/v1.66.2
kubectl annotate deckhouserelease v1.66.2 release.deckhouse.io/approved=true
kubectl wait --for=jsonpath='{.status.phase}'=Deployed deckhouserelease/v1.66.2


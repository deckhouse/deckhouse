#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Test AutoPatch update mode
echo "Run checks in AutoPatch update mode"

kubectl patch mc deckhouse --type=merge -p '{"spec": {"settings": {"releaseChannel": "Alpha", "update": {"mode": "AutoPatch"}}}}'

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
kubectl wait --for=jsonpath='{.status.message}'="Release is waiting for the 'release.deckhouse.io/approved: \"true\"' annotation" deckhouserelease/v1.66.0

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
kubectl apply -f - <<"EOF"
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  annotations:
    dryrun: "true"
  name: v1.65.3
spec:
  version: v1.65.3
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Superseded deckhouserelease/v1.65.0
kubectl wait --for=jsonpath='{.status.phase}'=Skipped deckhouserelease/v1.65.3
kubectl wait --for=jsonpath='{.status.phase}'=Deployed deckhouserelease/v1.65.5

kubectl annotate deckhouserelease v1.66.0 release.deckhouse.io/approved=true
kubectl wait --for=jsonpath='{.status.phase}'=Deployed deckhouserelease/v1.66.0



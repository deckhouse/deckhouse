#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

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

kubectl apply -f - <<"EOF"
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  annotations:
    dryrun: "true"
    release.deckhouse.io/suspended: "true"
  name: v1.65.5
spec:
  version: v1.65.5
EOF


kubectl wait --for=jsonpath='{.status.phase}'=Suspended deckhouserelease/v1.65.5

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

kubectl wait --for=jsonpath='{.status.phase}'=Pending deckhouserelease/v1.65.6

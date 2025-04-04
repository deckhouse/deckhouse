#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

kubectl patch mc deckhouse --type=merge -p '{"spec": {"settings": {"releaseChannel": "Alpha", "update": {"mode": "Auto", "windows": [{"from": "04:00", "to": "05:00"}], "notification": {"webhook": "https://foo-error.local", "minimalNotificationTime": "10h"}}}}}'

kubectl apply -f - <<"EOF"
apiVersion: deckhouse.io/v1alpha1
approved: false
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
kubectl wait --timeout=120s --for=jsonpath='{.status.message'}='Release is blocked, failed to send release notification' deckhouserelease/v1.66.0

#Copyright 2024 Flant JSC
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

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



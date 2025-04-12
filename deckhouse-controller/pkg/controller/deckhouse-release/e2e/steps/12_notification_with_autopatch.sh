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

token=$(curl -XPOST  -s https://webhook.site/token | jq -r .uuid)
echo "open https://webhook.site/#"'!'"/view/$token"

kubectl patch mc deckhouse --type=merge -p '{"spec": {"settings": {"releaseChannel": "Alpha", "update": {"mode": "AutoPatch", "windows": [{"from": "04:00", "to": "05:00"}], "notification": {"webhook": "https://webhook.site/'${token}'", "releaseType": "All", "minimalNotificationTime": "10h"}}}}}'

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


# open https://webhook.site/#!/view/29ef9a7e-4550-43a2-b04b-ed685ce6f3ce

kubectl apply -f - <<"EOF"
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  annotations:
    dryrun: "true"
  name: v1.65.1
spec:
  version: v1.65.1
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Pending deckhouserelease/v1.65.1
kubectl wait --for=jsonpath='{.metadata.annotations.release\.deckhouse\.io/notified}'=true deckhouserelease/v1.65.1
kubectl wait --for=jsonpath='{.metadata.annotations.release\.deckhouse\.io/notification-time-shift}'=true deckhouserelease/v1.65.1

# check notification webhook message
raw=$(curl -s https://webhook.site/token/$token/request/latest)
content=$(jq -r .content <<< "${raw}")


if echo "$content" | jq -e '.subject == "Deckhouse" and .version == "1.65.1" and (.message | startswith("New Deckhouse Release 1.65.1 is available. Release will be applied at:"))' > /dev/null; then
  echo "OK - webhook data exists"
else
  echo "Webhook data invalid: $content"
  exit 1;
fi

msg=$(kubectl get deckhouserelease/v1.65.1 -o jsonpath='{.status.message}')
if [[ "$msg" != Release\ is\ postponed,\ waiting\ for\ the\ update\ window\ until* ]]; then
	echo "Release message invalid: $msg"
	exit 1;
fi

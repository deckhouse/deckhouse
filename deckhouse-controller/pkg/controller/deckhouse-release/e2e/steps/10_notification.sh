#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Check notification

token=$(curl -XPOST  -s https://webhook.site/token | jq -r .uuid)
echo "open https://webhook.site/#"'!'"/view/$token"

kubectl patch mc deckhouse --type=merge -p '{"spec": {"settings": {"releaseChannel": "Alpha", "update": {"mode": "Auto", "windows": null, "notification": {"webhook": "https://webhook.site/'${token}'", "minimalNotificationTime": "10h"}}}}}'

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
  name: v1.66.0
spec:
  version: v1.66.0
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Pending deckhouserelease/v1.66.0
kubectl wait --for=jsonpath='{.metadata.annotations.release\.deckhouse\.io/notified}'=true deckhouserelease/v1.66.0
kubectl wait --for=jsonpath='{.metadata.annotations.release\.deckhouse\.io/notification-time-shift}'=true deckhouserelease/v1.66.0

# check notification webhook message
raw=$(curl -s https://webhook.site/token/$token/request/latest | jq -r .)
uuid=$(echo "$raw" | jq -r .uuid)
content=$(echo "$raw" | jq -r .content)

if echo "$content" | jq -e '.subject == "Deckhouse" and .version == "1.66.0" and (.message | startswith("New Deckhouse Release 1.66.0 is available"))' > /dev/null; then
  echo "OK - webhook data exists"
else
  echo "Webhook data invalid: $content"
  exit 1;
fi
# delete request
#curl -s -X DELETE https://webhook.site/token/29ef9a7e-4550-43a2-b04b-ed685ce6f3ce/request/$uuid > /dev/null
# stop check webhook

msg=$(kubectl get deckhouserelease/v1.66.0 -o jsonpath='{.status.message}')
if [[ "$msg" != Release\ is\ postponed\ until* ]]; then
	echo "Release message invalid: $msg"
	exit 1;
fi

#!/usr/bin/env bash

# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

nodeecho() {
        echo "NODE > " $@
}

cd "$(dirname "${BASH_SOURCE[0]}")"

UPMETER_COMMAND=$(kubectl -n d8-upmeter get sts upmeter -o json | jq -rc '.spec.template.spec.containers[0].command')
nodeecho "Storing upmeter pod command $UPMETER_COMMAND"

UPMETER_SERVICE_SELECTOR=$(kubectl -n d8-upmeter get svc upmeter -o json | jq -rc '.spec.selector')
nodeecho "Storing upmeter service selector $UPMETER_SERVICE_SELECTOR"

# Stop traffic and make pod do nothing while being awake
kubectl -n d8-upmeter get svc upmeter -o json | jq '.spec.selector={"app":"xupmeter"}' | kubectl apply -f -
kubectl -n d8-upmeter get sts upmeter -o json | jq '.spec.template.spec.containers[0].command=["sleep","infinity"]' | kubectl apply -f -
kubectl -n d8-upmeter delete po upmeter-0 --wait

VACUUM_SCRIPT="./__pod.sh"
REMOTE_VACUUM_SCRIPT="/db/vacuum.sh"
while true; do
        kubectl -n d8-upmeter -c upmeter cp "$VACUUM_SCRIPT" "upmeter-0:$REMOTE_VACUUM_SCRIPT" 2>/dev/null
        if [[ $? -eq 0 ]]; then
                break
        fi
        echo "waiting for the pod to start..."
        sleep 2
done

kubectl -n d8-upmeter -c upmeter exec -it upmeter-0 -- sh "$REMOTE_VACUUM_SCRIPT"

nodeecho "Restoring upmeter pod command $UPMETER_COMMAND"
kubectl -n d8-upmeter get sts upmeter -o json | jq ".spec.template.spec.containers[0].command=$UPMETER_COMMAND" | kubectl apply -f -

nodeecho "Restoring upmeter service selector $UPMETER_SERVICE_SELECTOR"
kubectl -n d8-upmeter get svc upmeter -o json | jq ".spec.selector=$UPMETER_SERVICE_SELECTOR" | kubectl apply -f -

nodeecho "Restarting pod..."
kubectl -n d8-upmeter delete po upmeter-0

## Back to normal.
## Note --origins=<# of masters>
# kubectl -n d8-upmeter get sts upmeter -o json | jq '.spec.template.spec.containers[0].command=["/upmeter", "start", "--origins=1"]' | kubectl apply -f - ; kubectl -n d8-upmeter get svc upmeter -o json | jq '.spec.selector={"app":"upmeter"}' | kubectl apply -f -

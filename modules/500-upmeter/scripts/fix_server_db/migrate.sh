#!/usr/bin/env bash

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

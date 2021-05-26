#!/usr/bin/env bash

#########################################################
#
# USAGE: ./migrate.sh [kubectl_context]
#
#########################################################

nodeecho() {
        echo "NODE > " $@
}

CONTEXT=${1:-kubernetes-admin@kubernetes}

UPMETER_COMMAND=$(kubectl --context=$CONTEXT -n d8-upmeter get sts upmeter -o json | jq -rc '.spec.template.spec.containers[0].command')
nodeecho "Storing upmeter pod command $UPMETER_COMMAND"

UPMETER_SERVICE_SELECTOR=$(kubectl --context=$CONTEXT -n d8-upmeter get svc upmeter -o json | jq -rc '.spec.selector')
nodeecho "Storing upmeter service selector $UPMETER_SERVICE_SELECTOR"

# Stop traffic and make pod do nothing while being awake
kubectl --context=$CONTEXT -n d8-upmeter get svc upmeter -o json | jq '.spec.selector={"app":"xupmeter"}' | kubectl --context=$CONTEXT apply -f -
kubectl --context=$CONTEXT -n d8-upmeter get sts upmeter -o json | jq '.spec.template.spec.containers[0].command=["sleep","infinity"]' | kubectl --context=$CONTEXT apply -f -
kubectl --context=$CONTEXT -n d8-upmeter delete po upmeter-0 --wait

VACUUM_SCRIPT="./__pod.sh"
REMOTE_VACUUM_SCRIPT="/db/vacuum.sh"
while true; do
        kubectl --context=$CONTEXT -n d8-upmeter -c upmeter cp "$VACUUM_SCRIPT" "upmeter-0:$REMOTE_VACUUM_SCRIPT" 2>/dev/null
        if [[ $? -eq 0 ]]; then
                break
        fi
        echo "waiting for the pod to start..."
        sleep 2
done

kubectl --context=$CONTEXT -n d8-upmeter -c upmeter exec -it upmeter-0 -- sh "$REMOTE_VACUUM_SCRIPT"

nodeecho "Restoring upmeter pod command $UPMETER_COMMAND"
kubectl --context=$CONTEXT -n d8-upmeter get sts upmeter -o json | jq ".spec.template.spec.containers[0].command=$UPMETER_COMMAND" | kubectl --context=$CONTEXT apply -f -

nodeecho "Restoring upmeter service selector $UPMETER_SERVICE_SELECTOR"
kubectl --context=$CONTEXT -n d8-upmeter get svc upmeter -o json | jq ".spec.selector=$UPMETER_SERVICE_SELECTOR" | kubectl --context=$CONTEXT apply -f -

nodeecho "Restarting pod..."
kubectl --context=$CONTEXT -n d8-upmeter delete po upmeter-0

## Back to normal.
## Note --origins=<# of masters>
# kubectl --context=$CONTEXT -n d8-upmeter get sts upmeter -o json | jq '.spec.template.spec.containers[0].command=["/upmeter", "start", "--origins=1"]' | kubectl --context=$CONTEXT apply -f - ; kubectl --context=$CONTEXT -n d8-upmeter get svc upmeter -o json | jq '.spec.selector={"app":"upmeter"}' | kubectl --context=$CONTEXT apply -f -

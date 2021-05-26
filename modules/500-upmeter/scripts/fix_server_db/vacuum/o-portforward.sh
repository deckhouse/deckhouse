CONTEXT=${1:-kubernetes-admin@kubernetes}
while true; do
        kubectl --context=$CONTEXT port-forward -n d8-upmeter upmeter-0 8091:8091 || sleep 2
done

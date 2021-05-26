CONTEXT=${1:-kubernetes-admin@kubernetes}
while true; do
        kubectl --context=$CONTEXT logs -f -n d8-upmeter upmeter-0 -c upmeter || sleep 2
done

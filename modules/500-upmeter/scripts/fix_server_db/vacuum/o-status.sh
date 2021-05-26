CONTEXT=${1:-kubernetes-admin@kubernetes}
watch kubectl --context=$CONTEXT -n d8-upmeter get po -l app=upmeter -o wide

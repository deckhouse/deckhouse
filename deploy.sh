#!/bin/bash

set -e

function Usage {
    echo
    echo "    $0 <gitlab-username> <gitlab-token> [-n <namespace (default antiopa)>] [-t <image-version (default master)>]"
    echo
}

if [[ $NAMESPACE == "" ]] ; then
    NAMESPACE="antiopa"
fi

if [[ $TAG == "" ]] ; then
    TAG="master"
fi

GITLAB_USERNAME=$1
if [[ $GITLAB_USERNAME == "" ]] ; then
    Usage
    exit 1
fi

GITLAB_TOKEN=$2
if [[ $GITLAB_TOKEN == "" ]] ; then
    Usage
    exit 1
fi

DOCKER_REGISTRY="registry.flant.com"
IMAGE="$DOCKER_REGISTRY/sys/antiopa"

dockercfgAuth=$(echo -n "$GITLAB_USERNAME:$GITLAB_TOKEN" | base64 -w0)
dockercfg=$(echo -n "{\"auths\": {\"$DOCKER_REGISTRY\": {\"auth\": \"$dockercfgAuth\", \"email\": \"some@email.com\"}}}" | base64 -w0)

mkdir -p /tmp/antiopa-deploy

cat > /tmp/antiopa-deploy/deploy.yaml <<EOF
apiVersion: v1
kind: Pod
metadata:
    name: deploy
spec:
    restartPolicy: Never
    containers:
        - name: deploy
          image: $IMAGE:$TAG
          command: ["bash", "-lec"]
          args: ["while true ; do sleep 1 ; done"]
          env:
            - name: TEST
              value: HELLO
    imagePullSecrets:
        - name: registrysecret
---
apiVersion: v1
kind: Secret
type: kubernetes.io/dockercfg
metadata:
  name: registrysecret
data:
  .dockercfg: $dockercfg
EOF

if [[ "$(kubectl get ns -a 2>/dev/null | cut -d' ' -f1 | grep "^$NAMESPACE\$")" == "" ]] ; then
    kubectl create ns $NAMESPACE
fi

if [[ "$(kubectl -n $NAMESPACE get pod -a 2>/dev/null | cut -d' ' -f1 | grep "^deploy\$")" != "" ]] ; then
    kubectl -n $NAMESPACE delete pod deploy

    while [[ "$(kubectl -n $NAMESPACE get pod -a 2>/dev/null | cut -d' ' -f1 | grep "^deploy\$")" != "" ]] ; do
        sleep 1
    done
fi

kubectl -n $NAMESPACE apply -f /tmp/antiopa-deploy/deploy.yaml
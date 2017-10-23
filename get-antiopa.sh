#!/bin/bash
set -e

# Antiopa installer
#   $ curl -fLOs https://github.com/deckhouse/deckhouse/raw/stable/get-antiopa.sh
#   $ sudo bash get-antiopa.sh

main() {
  NAMESPACE=${NAMESPACE:-'antiopa'}
  REGISTRY=${REGISTRY:-'registry.flant.com'}
  BRANCH=${BRANCH:-'stable'}
  IMAGE=${IMAGE:-"antiopa:antiopa-$BRANCH"}
  TOKEN=${TOKEN:-}
  DRY_RUN=${DRY_RUN:-0}
  OUT_FILE=${OUT_FILE:-}
  LOG_LEVEL=${LOG_LEVEL:-'DEBUG'}

  parse_args "$@" || (usage && exit 1)

  if [[ $REGISTRY == ":minikube" ]]; then
    REGISTRY="localhost:5000"
  fi

  MANIFESTS=
  generate_yaml

  if [[ -n $OUT_FILE ]]; then
    echo "$MANIFESTS" > $OUT_FILE
  fi

  if [[ $DRY_RUN == 1 ]]; then
    if [[ -z $OUT_FILE ]]; then
      echo "$MANIFESTS"
    fi
  else
    install_yaml
  fi

  return $?
}


usage() {
printf " Usage: $0 -n <namespace> --token <gitlab user auth token> [--tag <image tag>] [--dry-run]

    -n, --namespace <namespace>
            Define kubernetes namespace.
            Default: antiopa

    --registry <docker registry url>
            URL of registry with antiopa image
            Default: registry.flant.com

    --image <name:tag>
            Antiopa image name. Use it for
            Default: antiopa:stable

    --token <token>
            Auth token generated in gitlab user's profile.
            If no token specified, no imagePullSecret will generate.
            E.g. registry in minikube can be without auth (dapp kube minikube setup).

    -o, --out <filename>
            Put generated yaml into file.

    --dry-run
            Do not run kubectl apply.
            Print yaml to stdout or to -o file.

    --log-level <INFO|ERROR|DEBUG>
            Set RLOG_LOG_LEVEL.
            Default: INFO

    --help|-h
            Print this message.

"
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      -n|--namespace)
        NAMESPACE="$2"
        shift
        ;;
      --registry)
        REGISTRY="$2"
        shift;;
      --image)
        IMAGE="$2"
        shift
        ;;
      --token)
        TOKEN="$2"
        shift
        ;;
      -o|--out)
        OUT_FILE="$2"
        shift
        ;;
      --dry-run)
        DRY_RUN=1
        ;;
      --log-level)
        LOG_LEVEL="$2"
        shift
        ;;
      --help|-h)
        return 1
        ;;
      --*)
        echo "Illegal option $1"
        return 1
        ;;
    esac
    shift $(( $# > 0 ? 1 : 0 ))
  done
}

generate_yaml() {
  local SECRET
  local IMAGE_PULL_SECRETS

  if [ "x$TOKEN" != "x" ]
  then

    local AUTH_CFG_BASE64=$(cat <<- JSON | base64 -w0
{"$REGISTRY": {
  "username": "oauth2",
  "password": "${TOKEN}",
  "auth": "$(echo -n "oauth2:${TOKEN}" | base64 -w0)",
  "email": "some@email.com"
 }
}
JSON
)

    SECRET=$(cat <<- YAML
---
apiVersion: v1
kind: Secret
type: kubernetes.io/dockercfg
metadata:
  name: registrysecret
data:
  .dockercfg: ${AUTH_CFG_BASE64}
YAML
)
    IMAGE_PULL_SECRETS=$(cat <<- YAML
      imagePullSecrets:
        - name: registrysecret
YAML
)

  fi


  local DEPLOYMENT=$(cat <<- YAML
---
apiVersion: v1
kind: Secret
metadata:
  name: antiopa-secret
data:
  gitlab-token: $(echo -n ${TOKEN} | base64 -w0)
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: antiopa
spec:
  replicas: 1
  template:
    metadata:
      labels:
        service: antiopa
    spec:
      containers:
        - name: antiopa
          image: ${REGISTRY}/${IMAGE}
          imagePullPolicy: Always
          command: ["/antiopa/antiopa"]
          workingDir: /antiopa
          env:
            - name: KUBERNETES_DEPLOYED
              value: "$(date --rfc-3339=seconds)"
            - name: RLOG_LOG_LEVEL
              value: ${LOG_LEVEL}
            - name: GITLAB_TOKEN
              valueFrom:
                secretKeyRef:
                  name: antiopa-secret
                  key: gitlab-token
      serviceAccount: antiopa
${IMAGE_PULL_SECRETS}
YAML
)

  local RBAC=$(cat <<- YAML
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: antiopa
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: Role
metadata:
  name: antiopa
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: RoleBinding
metadata:
  name: antiopa
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: antiopa
subjects:
  - kind: ServiceAccount
    name: antiopa
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: antiopa-cluster
rules:
- apiGroups:
    - "*"
  resources:
    - "*"
  verbs:
    - "*"
- nonResourceURLs:
    - "*"
  verbs:
    - "*"
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: antiopa-cluster-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: antiopa-cluster
subjects:
  - kind: ServiceAccount
    name: antiopa
    namespace: ${NAMESPACE}

YAML
)

  local VALUES_CONFIG_MAP=$(cat <<- YAML
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: antiopa
data:
  help-example: |
    ---
    section 'help-example' section is not nedded. You can remove it after adding real values.
    ConfigMap 'antiopa' contains global values under 'values' key and module specific values
    under <module>-values keys.
    Antiopa will add keys <module>-checksum. Deletion of this keys will re-run modules.
    ---
    apiVersion: v1
    kind: ConfigMap
    metadata:
    name: antiopa
    data:
      values: |
	      go:
	      - 2
	    test1-values: |
	      go:
	      - 4
	      - 6
	      - 7
	    test2-values: |
	      key: value
YAML
)

  MANIFESTS=$(cat << YAML
$SECRET
$DEPLOYMENT
$RBAC
$VALUES_CONFIG_MAP
YAML
)

  return 0

}

install_yaml() {
  if [ $DRY_RUN == 0 ]; then

    echo Begin install

    if [[ "$(kubectl get ns -a 2>/dev/null | cut -d' ' -f1 | grep "^$NAMESPACE\$")" == "" ]] ; then
        echo "  " Create namespace $NAMESPACE
        kubectl create ns $NAMESPACE
    fi

#    if [[ "$(kubectl -n $NAMESPACE get pod -a 2>/dev/null | cut -d' ' -f1 | grep "^deploy\$")" != "" ]] ; then
#        echo "  " Delete Install manifests...
#        kubectl -n $NAMESPACE delete pod deploy
#
#        while [[ "$(kubectl -n $NAMESPACE get pod -a 2>/dev/null | cut -d' ' -f1 | grep "^deploy\$")" != "" ]] ; do
#            sleep 1
#        done
#    fi

    echo "  " Apply manifests
    echo "$MANIFESTS" | kubectl -n $NAMESPACE apply -f -

    echo End install
  fi
  return $?
}

# wait for full file download if executed as
# $ curl | sh
main "$@"

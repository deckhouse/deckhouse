ARG BASE_ALPINE
FROM $BASE_ALPINE
ADD "https://storage.googleapis.com/kubernetes-release/release/v1.21.14/bin/linux/amd64/kubectl" /bin/
ADD "https://github.com/flant/kube-resource-unit-converter/releases/download/0.1/kube-resource-unit-converter" /bin/
ADD loop /bin/
RUN apk add --no-cache bash grep coreutils curl bc jq && chmod +x /bin/kubectl /bin/kube-resource-unit-converter
ENTRYPOINT ["/bin/loop"]

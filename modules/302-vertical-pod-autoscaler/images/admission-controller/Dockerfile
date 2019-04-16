FROM golang:1.12-alpine3.9
WORKDIR /go/src/k8s.io/autoscaler
RUN wget https://github.com/kubernetes/autoscaler/archive/vertical-pod-autoscaler-0.5.0.tar.gz -O - | tar -xz --strip-components=1 && \
    apk --update add bash make git
ADD apply-limits.patch .
RUN patch -p1 < apply-limits.patch && \
    cd vertical-pod-autoscaler/pkg/admission-controller && \
    make build-binary

FROM alpine:3.9
COPY --from=0 /go/src/k8s.io/autoscaler/vertical-pod-autoscaler/pkg/admission-controller/admission-controller /bin/
ENTRYPOINT admission-controller --v=4 --stderrthreshold=info

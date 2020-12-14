# https://github.com/kubernetes/autoscaler/pull/3030
ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
WORKDIR /src/
COPY recommender.patch ./
RUN apk add --no-cache git mercurial patch && \
    wget https://codeload.github.com/kubernetes/autoscaler/tar.gz/vertical-pod-autoscaler/v0.9.0 -O - | tar -xz --strip-components=1 -C /src/ && \
    patch -p1 < /src/recommender.patch && \
    cd vertical-pod-autoscaler/ && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o recommender pkg/recommender/main.go

FROM $BASE_ALPINE
COPY --from=0 /src/vertical-pod-autoscaler/recommender /recommender
ENTRYPOINT [ "/recommender" ]

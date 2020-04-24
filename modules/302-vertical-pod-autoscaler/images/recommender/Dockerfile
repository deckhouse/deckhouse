# https://github.com/kubernetes/autoscaler/pull/3030
FROM golang:1.14-alpine3.11
WORKDIR /src/
RUN apk add --no-cache git mercurial patch && \
    wget https://github.com/flant/autoscaler/archive/57b2ef8134552093cc6ac07ce97d28a29dd1674e.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    cd vertical-pod-autoscaler/ && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o recommender pkg/recommender/main.go

FROM alpine:3.11
COPY --from=0 /src/vertical-pod-autoscaler/recommender /recommender
ENTRYPOINT [ "/recommender" ]

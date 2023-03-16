ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE AS build
WORKDIR /src
RUN wget https://github.com/aquasecurity/k8s-node-collector/archive/refs/tags/v0.0.5.tar.gz -O - | tar -xz --strip-components=1
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w -extldflags "-static"' -o node-collector ./cmd/node-collector/main.go

FROM $BASE_ALPINE
COPY --from=build /src/node-collector /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/node-collector" ]
ENV LANG=C.UTF-8

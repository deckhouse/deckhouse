ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE AS build
WORKDIR /src
RUN apk add patch
RUN wget https://github.com/aquasecurity/trivy-operator/archive/refs/tags/v0.11.1.tar.gz -O - | tar -xz --strip-components=1

COPY patches/001-add-registry-secret-as-dockerconfigjson.patch /src
RUN patch -p1 < 001-add-registry-secret-as-dockerconfigjson.patch

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w -extldflags "-static"' -o operator-trivy ./cmd/trivy-operator/main.go

FROM $BASE_ALPINE
COPY --from=build /src/operator-trivy /
ENTRYPOINT [ "/operator-trivy" ]
ENV LANG=C.UTF-8

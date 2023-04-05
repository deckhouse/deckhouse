ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE AS build
WORKDIR /src
RUN apk add patch
RUN wget https://github.com/aquasecurity/trivy-operator/archive/refs/tags/v0.12.1.tar.gz -O - | tar -xz --strip-components=1 && \
    go mod vendor

COPY patches/001-add-registry-secret-as-dockerconfigjson.patch /src
RUN patch -p1 < 001-add-registry-secret-as-dockerconfigjson.patch

COPY patches/002-fix-defseq-lib-for-cis-compability.patch /src
RUN patch -p1 < 002-fix-defseq-lib-for-cis-compability.patch

COPY patches/003-fix-defseq-lib-for-deckhouse-setup.patch /src
RUN patch -p1 < 003-fix-defseq-lib-for-deckhouse-setup.patch

RUN CGO_ENABLED=0 GOOS=linux go build -mod vendor -ldflags '-s -w -extldflags "-static"' -o operator-trivy ./cmd/trivy-operator/main.go

FROM $BASE_ALPINE
COPY --from=build /src/operator-trivy /
ENTRYPOINT [ "/operator-trivy" ]
ENV LANG=C.UTF-8

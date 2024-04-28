ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE_DEV

FROM $BASE_GOLANG_20_ALPINE_DEV AS build
ARG GOPROXY
ARG SOURCE_REPO
ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src
RUN git clone --depth 1 --branch v0.16.4 ${SOURCE_REPO}/aquasecurity/trivy-operator.git .

# some patches are applied to dependencies
RUN go mod vendor

COPY patches/001-add-registry-secret-as-dockerconfigjson.patch /src
COPY patches/002-skip-some-checks.patch /src
COPY patches/003-aws-ecr.patch /src
COPY patches/004-scan-job-registry-ca.patch /src
COPY patches/005-cis-benchmark-on-startup.patch /src
COPY patches/006-new-metrics.patch /src

RUN patch -p1 < 001-add-registry-secret-as-dockerconfigjson.patch && \
    patch -p1 < 002-skip-some-checks.patch && \
    patch -p1 < 003-aws-ecr.patch && \
    patch -p1 < 004-scan-job-registry-ca.patch && \
    patch -p1 < 005-cis-benchmark-on-startup.patch && \
    patch -p1 < 006-new-metrics.patch

RUN go get golang.org/x/net@v0.17.0 && go mod tidy && go mod vendor
RUN go build -ldflags '-s -w -extldflags "-static"' -o operator-trivy ./cmd/trivy-operator/main.go

RUN chown 64535:64535 operator-trivy
RUN chmod 0700 operator-trivy

FROM $BASE_DISTROLESS
COPY --from=build /src/operator-trivy /
ENTRYPOINT [ "/operator-trivy" ]

ARG BASE_GOLANG_22_ALPINE_DEV
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_22_ALPINE_DEV as build

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src
COPY patches/01-add-custom-ca.patch /

RUN git clone --depth 1 --branch v1.3.1 https://github.com/ratify-project/ratify . && \
patch -p1 < /01-add-custom-ca.patch && \
go build -ldflags="-s -w" -o ratify cmd/ratify/main.go

RUN chown 64535:64535 ratify
RUN chmod 0700 ratify

FROM $BASE_DISTROLESS
COPY --from=build /src/ratify /bin/ratify
ENTRYPOINT ["/bin/ratify"]

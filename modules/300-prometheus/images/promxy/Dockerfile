ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE as artifact

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN apk add --no-cache make git openssh-client
RUN mkdir -p ~/.ssh && echo "StrictHostKeyChecking accept-new" > ~/.ssh/config
RUN --mount=type=ssh git clone --depth 1 --branch v0.0.84 ${SOURCE_REPO}/jacksontj/promxy.git /promxy
WORKDIR /promxy/

RUN go build -o promxy ./cmd/promxy  && \
    chown -R 64535:64535 /promxy/ && \
    chmod 0700 /promxy/promxy


FROM $BASE_DISTROLESS

COPY --from=artifact /promxy/promxy /bin/promxy

ENTRYPOINT ["/bin/promxy"]

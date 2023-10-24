ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE as builder

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app
COPY . .
RUN go build -ldflags="-s -w" -o doc_builder .

RUN chown 64535:64535 doc_builder
RUN chmod 0700 doc_builder

FROM $BASE_DISTROLESS
COPY --from=builder /app/doc_builder /app/doc_builder
CMD ["/app/doc_builder"]

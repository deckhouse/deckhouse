ARG BASE_DISTROLESS
ARG BASE_GOLANG_21_ALPINE

FROM $BASE_GOLANG_21_ALPINE as builder

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app
COPY . .
RUN go build -ldflags="-s -w" -o policy_validator .

RUN chown 64535:64535 policy_validator
RUN chmod 0700 policy_validator

FROM $BASE_DISTROLESS
COPY --from=builder /app/policy_validator /app/policy_validator
CMD ["/app/policy_validator"]

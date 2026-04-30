FROM golang:1.25-alpine AS builder

# Versions are injected from the chart Makefile via docker build --build-arg.
# See version in Makefile in root of git repo (for deckhouse, not for this module)

ARG OPA_VERSION
ARG GATOR_VERSION

RUN apk add --no-cache ca-certificates git

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOBIN=/usr/local/bin

# Build CLIs used by constraint test workflows.
RUN go install github.com/open-policy-agent/opa@${OPA_VERSION}
RUN go install github.com/open-policy-agent/gatekeeper/v3/cmd/gator@${GATOR_VERSION}

WORKDIR /work/deckhouse
COPY . .
RUN go build -o /usr/local/bin/constraint_testgen ./modules/015-admission-policy-engine/charts/constraint-templates/tests/tools/constraint_testgen

FROM alpine:3.22

RUN apk add --no-cache bash ca-certificates

COPY --from=builder /usr/local/bin/opa /usr/local/bin/opa
COPY --from=builder /usr/local/bin/gator /usr/local/bin/gator
COPY --from=builder /usr/local/bin/constraint_testgen /usr/local/bin/constraint_testgen

WORKDIR /work/deckhouse/modules/015-admission-policy-engine/charts/constraint-templates/tests

CMD ["bash"]

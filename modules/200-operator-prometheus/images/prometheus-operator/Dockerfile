ARG BASE_DISTROLESS
ARG BASE_GOLANG_18_ALPINE

FROM $BASE_GOLANG_18_ALPINE as artifact

RUN apk add --no-cache make git
RUN git clone --depth 1 --branch v0.62.0 https://github.com/prometheus-operator/prometheus-operator.git /prometheus-operator
WORKDIR /prometheus-operator/

RUN make operator

FROM $BASE_DISTROLESS

COPY --from=artifact /prometheus-operator/operator /bin/

ENTRYPOINT ["/bin/operator"]

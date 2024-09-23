ARG BASE_GOLANG_21_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_21_ALPINE as artifact
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o render-etc-hosts-with-cluster-domain-aliases main.go

FROM $BASE_DISTROLESS
COPY --from=artifact /src/render-etc-hosts-with-cluster-domain-aliases /
ENTRYPOINT [ "/render-etc-hosts-with-cluster-domain-aliases" ]

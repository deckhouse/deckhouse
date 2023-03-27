ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags='-extldflags "-static" -s -w' -o render-etc-hosts-with-cluster-domain-aliases main.go


FROM scratch
COPY --from=artifact /src/render-etc-hosts-with-cluster-domain-aliases /
ENTRYPOINT [ "/render-etc-hosts-with-cluster-domain-aliases" ]

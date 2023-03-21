ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE AS build
WORKDIR /src
COPY . .
RUN apk add unzip curl
RUN curl --cacert trusted_sub_ca.crt --user-agent 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36' \
        https://bdu.fstec.ru/files/documents/vulxml.zip -o vulxml.zip
RUN unzip vulxml.zip
RUN go run converter/main.go export/export.xml bdu.gob
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w -extldflags "-static"' -o bdu-exporter main.go

FROM $BASE_ALPINE
COPY --from=build /src/bdu-exporter /bdu-exporter
COPY --from=build /src/bdu.gob /bdu.gob
ENTRYPOINT [ "/bdu-exporter" ]
CMD [ "/bdu.gob" ]

ENV LANG=C.UTF-8

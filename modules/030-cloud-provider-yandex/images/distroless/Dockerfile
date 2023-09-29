ARG BASE_SCRATCH
ARG BASE_ALPINE

FROM $BASE_ALPINE as apk_extractor

WORKDIR /workdir


RUN apk update
RUN apk add rsync ca-certificates
RUN apk fetch -R -o . tzdata alpine-baselayout-data ca-certificates

RUN mkdir /apk && for apk in *.apk; do tar -xzvf "$apk" -C /apk; done

COPY file-filter .
RUN mkdir /filtered-apk && rsync -av --include-from=file-filter --include='*/' --exclude '*' /apk/ /filtered-apk/

RUN update-ca-certificates && mkdir -p /filtered-apk/etc/ssl/certs && cp -r /etc/ssl/cert.pem /etc/ssl/certs /filtered-apk/etc/ssl/

RUN echo "deckhouse:x:64535:64535:deckhouse:/:/sbin/nologin" >> /filtered-apk/etc/passwd
RUN echo "deckhouse:x:64535:" >> /filtered-apk/etc/group
RUN echo "deckhouse:!::0:::::" >> /filtered-apk/etc/shadow

FROM $BASE_SCRATCH

COPY --from=apk_extractor /filtered-apk/ /

FROM alpine:3.18 as apk_extractor

WORKDIR /workdir


RUN apk update
RUN apk add rsync
RUN apk fetch -R -o . tzdata alpine-baselayout-data

RUN mkdir /apk && for apk in *.apk; do tar -xzvf "$apk" -C /apk; done

COPY file-whitelist .
RUN mkdir /filtered-apk && rsync --verbose -av --include-from=file-whitelist --include='*/' --exclude '*' /apk/ /filtered-apk/

FROM scratch

COPY --from=apk_extractor /filtered-apk/ /

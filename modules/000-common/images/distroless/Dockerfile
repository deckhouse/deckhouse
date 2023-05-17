ARG BASE_SCRATCH
ARG BASE_ALPINE

FROM $BASE_ALPINE as apk_extractor

WORKDIR /workdir


RUN apk update
RUN apk add rsync
RUN apk fetch -R -o . tzdata alpine-baselayout-data

RUN mkdir /apk && for apk in *.apk; do tar -xzvf "$apk" -C /apk; done

COPY file-whitelist .
RUN mkdir /filtered-apk && rsync -av --include-from=file-whitelist --include='*/' --exclude '*' /apk/ /filtered-apk/

FROM $BASE_SCRATCH

COPY --from=apk_extractor /filtered-apk/ /

ARG BASE_ALPINE

FROM $BASE_ALPINE as artifact
# install curl
RUN apk add --update curl
# download syncer
RUN curl -sSfL https://github.com/AliyunContainerService/image-syncer/releases/download/v1.3.0/image-syncer-v1.3.0-linux-amd64.tar.gz \
  | tar -xzf -
ADD /copy-images.sh /copy-images.sh
RUN chmod 755 /copy-images.sh /image-syncer

FROM $BASE_ALPINE
# jq and bash need for script, coreutils for mktemp with suffix
RUN apk add --update --no-cache bash jq coreutils
COPY --from=artifact /image-syncer /usr/local/bin/image-syncer
COPY --from=artifact /copy-images.sh  /usr/local/bin/copy-images.sh


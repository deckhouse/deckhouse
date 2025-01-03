ARG DISTRO=ruby:3.1-alpine
FROM $DISTRO

ARG TOOL_VERSION
RUN apk add --no-cache autoconf bison bzip2 bzip2-dev ca-certificates coreutils dpkg-dev dpkg gcc gdbm-dev glib-dev libc-dev libffi-dev \
     libxml2-dev libxslt-dev linux-headers make ncurses-dev	openssl	openssl-dev patch	procps readline-dev	tar	xz yaml-dev	zlib-dev \
     libcurl xz-libs g++
RUN gem install html-proofer -v ${TOOL_VERSION} --no-document
VOLUME ["/src"]
WORKDIR /src
ENTRYPOINT ["htmlproofer"]

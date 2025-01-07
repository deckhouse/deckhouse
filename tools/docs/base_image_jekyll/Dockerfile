ARG DISTRO=ruby:3.1-alpine
FROM $DISTRO
ADD Gemfile /
ADD Gemfile.lock /

RUN addgroup -g 1000 jekyll && adduser -u 1000 -G jekyll -DH jekyll && \
    apk add --no-cache autoconf bison bzip2 bzip2-dev ca-certificates coreutils dpkg-dev dpkg gcc gdbm-dev glib-dev libc-dev libffi-dev \
        libxml2-dev libxslt-dev linux-headers make ncurses-dev	openssl	openssl-dev patch	procps readline-dev	tar	xz yaml-dev	zlib-dev \
        libcurl xz-libs g++ rsync bash jq

RUN YQ4_VERSION=v4.9.6 && wget https://github.com/mikefarah/yq/releases/download/${YQ4_VERSION}/yq_linux_amd64 -O /usr/bin/yq && chmod +x /usr/bin/yq

RUN gem install bundler -v 2.6.2  --no-document && bundle install

WORKDIR /srv/jekyll-data/
ENTRYPOINT ["/bin/bash"]

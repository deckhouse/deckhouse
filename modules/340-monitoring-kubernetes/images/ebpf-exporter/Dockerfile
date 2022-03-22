ARG BASE_UBUNTU
ARG BASE_GOLANG_17_ALPINE

FROM $BASE_GOLANG_17_ALPINE as kernel-version-parser

WORKDIR /go/src/
COPY kernel-version-parser .
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -s -extldflags "-static"' -o kernel-version-parser main.go


FROM $BASE_UBUNTU

RUN apt-get update && apt-get -y install wget bison build-essential cmake flex git libedit-dev \
  libllvm6.0 llvm-6.0-dev libclang-6.0-dev python zlib1g-dev libelf-dev libfl-dev python3-distutils

RUN apt-get clean autoclean && apt-get autoremove --yes && rm -rf /var/lib/{apt,dpkg,cache,log}/

RUN mkdir bcc && cd bcc && \
    wget -O - https://github.com/iovisor/bcc/releases/download/v0.23.0/bcc-src-with-submodule.tar.gz | tar -xz --strip-components=1 && \
    mkdir build; cd build  && \
    cmake ..  && \
    make  && \
    make install

RUN rm -rf bcc/
RUN apt-get purge -y bison cmake flex libedit-dev libllvm6.0 llvm-6.0-dev libclang-6.0-dev zlib1g-dev libelf-dev python3-distutils libfl-dev

RUN wget -O /usr/local/bin/ebpf_exporter https://github.com/cloudflare/ebpf_exporter/releases/download/v1.2.5/ebpf_exporter.x86_64 && \
  chmod +x /usr/local/bin/ebpf_exporter
RUN apt-get purge -y wget

COPY --from=kernel-version-parser /go/src/kernel-version-parser /usr/local/bin/
COPY entrypoint.sh /

ENTRYPOINT [ "/entrypoint.sh" ]

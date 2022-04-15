ARG BASE_UBUNTU

FROM $BASE_UBUNTU

RUN apt-get update && apt-get -y install wget bison build-essential cmake flex git libedit-dev \
    libllvm6.0 llvm-6.0-dev libclang-6.0-dev python zlib1g-dev libelf-dev libfl-dev python3-distutils && \

    apt-get clean autoclean && apt-get autoremove --yes && rm -rf /var/lib/{apt,dpkg,cache,log}/ && \

    mkdir bcc && cd bcc && \
    wget -O - https://github.com/iovisor/bcc/releases/download/v0.23.0/bcc-src-with-submodule.tar.gz | tar -xz --strip-components=1 && \
    mkdir build; cd build  && \
    cmake ..  && \
    make  && \
    make install && \

    rm -rf bcc/ && \
    apt-get purge -y bison cmake flex libedit-dev libllvm6.0 llvm-6.0-dev libclang-6.0-dev zlib1g-dev libelf-dev python3-distutils libfl-dev

RUN wget -O /usr/local/bin/ebpf_exporter https://github.com/cloudflare/ebpf_exporter/releases/download/v1.2.5/ebpf_exporter.x86_64 && \
    chmod +x /usr/local/bin/ebpf_exporter && \
    apt-get purge -y wget

ENTRYPOINT ["/usr/local/bin/ebpf_exporter"]

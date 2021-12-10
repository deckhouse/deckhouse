ARG BASE_ALPINE
FROM $BASE_ALPINE as artifact

ARG VERSION=0.0.21
ARG COMMIT_REF=755e331a276d4dd26f84377db5504a38109df8fb

RUN apk add --no-cache go git
RUN mkdir /src
RUN git clone https://github.com/rancher/local-path-provisioner.git /src
WORKDIR /src
RUN git checkout "${COMMIT_REF}"

# Do not create directory if not found (GH: #224)
COPY patches/fix-directory-or-create.patch /
RUN git apply /fix-directory-or-create.patch

RUN CGO_ENABLED=0 go build -ldflags "-X main.VERSION=${VERSION} -extldflags -static -s -w" -o /local-path-provisioner

FROM $BASE_ALPINE

RUN apk add --no-cache ca-certificates \
                       e2fsprogs \
                       findmnt \
                       xfsprogs \
                       blkid \
                       e2fsprogs-extra

COPY --from=artifact /local-path-provisioner /usr/bin/local-path-provisioner

ENTRYPOINT ["/usr/bin/local-path-provisioner"]

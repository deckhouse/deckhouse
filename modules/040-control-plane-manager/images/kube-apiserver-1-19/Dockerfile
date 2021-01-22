ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE

FROM $BASE_GOLANG_ALPINE
ARG KUBERNETES_VERSION=v1.19.5

WORKDIR /src/
COPY old_certs.patch /

RUN apk add --no-cache make bash git mercurial patch rsync && \
    wget https://github.com/kubernetes/kubernetes/archive/${KUBERNETES_VERSION}.tar.gz -O - | \
    tar -xz --strip-components=1 -C /src/ && \
    patch -p0 < /old_certs.patch

ENV KUBE_GIT_VERSION_FILE=.kube-version
RUN echo "KUBE_GIT_VERSION='${KUBERNETES_VERSION}-flant.$(ls /*.patch | wc -l)'"   >> $KUBE_GIT_VERSION_FILE && \
    echo "KUBE_GIT_MAJOR='$(echo ${KUBERNETES_VERSION} | tr -d v | cut -d. -f 1)'" >> $KUBE_GIT_VERSION_FILE && \
    echo "KUBE_GIT_MINOR='$(echo ${KUBERNETES_VERSION} | tr -d v | cut -d. -f 2)'" >> $KUBE_GIT_VERSION_FILE && \
    echo "KUBE_GIT_COMMIT='0000000000000000000000000000000000000000'"              >> $KUBE_GIT_VERSION_FILE && \
    echo "KUBE_GIT_TREE_STATE='archive'"                                           >> $KUBE_GIT_VERSION_FILE
RUN make all WHAT=cmd/kube-apiserver


FROM $BASE_ALPINE
COPY --from=0 /src/_output/bin/kube-apiserver /usr/local/bin/kube-apiserver
COPY --from=k8s.gcr.io/pause:3.1 /pause /pause
ENTRYPOINT [ "/usr/local/bin/kube-apiserver" ]

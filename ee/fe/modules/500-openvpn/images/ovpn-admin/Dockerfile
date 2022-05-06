ARG BASE_ALPINE
ARG BASE_NODE_16_ALPINE
ARG BASE_GOLANG_17_ALPINE

FROM $BASE_ALPINE as src
WORKDIR /app
ENV GIT_COMMIT=53119e17b2553981207703fb98eadf7bb96570f4
RUN apk update && apk add git && git clone https://github.com/flant/ovpn-admin.git . && git checkout $GIT_COMMIT && echo $GIT_COMMIT > version

FROM $BASE_NODE_16_ALPINE AS frontend
WORKDIR /app
COPY --from=src /app /app
RUN  cd frontend && npm install && npm run build

FROM $BASE_GOLANG_17_ALPINE AS backend
WORKDIR /app
COPY --from=src /app /app
RUN go build .

FROM $BASE_ALPINE
WORKDIR /app
RUN apk update && apk add bash openssl openvpn
RUN echo $GIT_COMMIT > /app/version
COPY --from=backend /app/ovpn-admin /app/version /app/
COPY --from=frontend /app/frontend/static /app/frontend/static
COPY client.conf.tpl ccd.tpl /app/templates/
COPY rootfs /
ENV LANG=C.UTF-8
ENV GIT_COMMIT=$GIT_COMMIT

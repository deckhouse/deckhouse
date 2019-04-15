FROM python:3.6-alpine

RUN apk add curl git gcc libc-dev linux-headers npm openssl

RUN python3.6 -m pip install flask uwsgi

WORKDIR /app

RUN cd /app && \
    git clone -b 0.0.6 https://github.com/vitaliy-sn/openvpn-easyrsa-web-ui.git . && \
    export NPM_CONFIG_PREFIX=/root/.npm && \
    cd /app/frontend && \
    npm install && \
    npm run build && \
    rm -rf /app/frontend/node_modules /root/.npm /app/.git

RUN apk del gcc libc-dev linux-headers npm

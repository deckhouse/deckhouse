FROM python:3.6-alpine3.8
WORKDIR /app
ADD src /app
RUN apk add --no-cache --virtual .build-deps build-base libffi-dev openssl-dev && \
    pip3 install -r /app/requirements.txt && \
    apk del .build-deps

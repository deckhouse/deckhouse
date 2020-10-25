FROM python:3.7.9-alpine3.12
WORKDIR /app
ADD src /app
RUN apk add --no-cache --virtual .build-deps build-base libffi-dev openssl-dev && \
    pip3 install -r /app/requirements.txt && \
    apk del .build-deps

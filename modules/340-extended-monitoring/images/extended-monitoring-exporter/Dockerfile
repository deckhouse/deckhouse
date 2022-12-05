ARG BASE_PYTHON_ALPINE
FROM $BASE_PYTHON_ALPINE
WORKDIR /app
ADD src /app
RUN apk add --no-cache --virtual .build-deps build-base libffi-dev openssl-dev && \
    pip3 install -r /app/requirements.txt && \
    apk del .build-deps
ENTRYPOINT ["python3"]

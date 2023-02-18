ARG BASE_SHELL_OPERATOR
FROM $BASE_SHELL_OPERATOR
COPY requirements.txt requirements.txt

RUN apk add --no-cache curl python3 py3-pip && \
    pip3 install -r requirements.txt

COPY hooks/ /hooks

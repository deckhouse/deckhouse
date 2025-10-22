ARG DISTRO=python:3.7.16-alpine3.16
FROM $DISTRO

ARG TOOL_VERSION
ENV LANG=ru_RU.UTF-8
ENV LANGUAGE=ru_RU:ru
ENV LC_LANG=ru_RU.UTF-8
ENV LC_ALL=ru_RU.UTF-8
ENV LC_MESSAGES=ru_RU.UTF-8
ENV TZ=Europe/Moscow
ENV PIP_ROOT_USER_ACTION=ignore

RUN apk add --no-cache bash hunspell=${TOOL_VERSION} && pip3 install pyyaml==6.0.1

ENTRYPOINT ["/bin/bash"]
WORKDIR /spelling

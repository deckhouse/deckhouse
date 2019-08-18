FROM python:3.7.4-alpine
RUN pip install ruamel.yaml
COPY rootfs /
WORKDIR /app
ENTRYPOINT ["python3", "/app/user-authz-webhook-configurator.py"]

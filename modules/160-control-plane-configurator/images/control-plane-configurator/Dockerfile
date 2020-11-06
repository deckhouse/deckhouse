FROM python:3.7.9-alpine3.12
RUN pip install ruamel.yaml
COPY control-plane-configurator.py /
ENTRYPOINT ["python3", "/control-plane-configurator.py"]

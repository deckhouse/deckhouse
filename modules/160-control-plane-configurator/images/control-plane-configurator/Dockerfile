FROM python:3.7.4-alpine
RUN pip install ruamel.yaml
COPY control-plane-configurator.py /
ENTRYPOINT ["python3", "/control-plane-configurator.py"]

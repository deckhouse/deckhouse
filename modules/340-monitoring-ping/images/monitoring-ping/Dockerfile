FROM python:3.6-alpine3.8
COPY monitoring-ping.py requirements.txt /
WORKDIR /
RUN pip3 install --upgrade pip && pip3 install -r requirements.txt && apk add --no-cache fping
ENTRYPOINT ["python3", "/monitoring-ping.py"]

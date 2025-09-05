FROM riptide01/shell-operator:v0
RUN apk --no-cache add python3 py3-pip
COPY shell-test/requirements.txt requirements.txt
RUN pip install -r requirements.txt --break-system-packages
ADD hooks /hooks
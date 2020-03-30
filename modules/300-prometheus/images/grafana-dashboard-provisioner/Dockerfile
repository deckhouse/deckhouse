FROM flant/shell-operator:v1.0.0-beta.8-alpine3.9
COPY hooks/ /hooks
RUN apk add --no-cache bash jq curl sqlite && \
  curl https://slugify.now.sh > slugify && \
  chmod +x slugify && \
  mv slugify /usr/local/bin/

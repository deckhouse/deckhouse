FROM flant/shell-operator:v1.0.0-beta.10-alpine3.11
COPY hooks/ /hooks
RUN apk add --no-cache bash jq curl sqlite && \
  curl https://slugify.now.sh > slugify && \
  chmod +x slugify && \
  mv slugify /usr/local/bin/

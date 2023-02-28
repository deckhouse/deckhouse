ARG BASE_NGINX_ALPINE

FROM node:14-alpine3.12@sha256:426384fb33a11d27dbbdc545f39bb8daacd3e7db7c60b52cd6bc0597e0045b8d as artifact
# dependencies for favicon generator
RUN apk add python2 python3 vips make build-base
RUN mkdir -p /app
WORKDIR /app
COPY package.json /app/
COPY yarn.lock /app/
RUN yarn install
COPY . /app
RUN yarn run build

FROM $BASE_NGINX_ALPINE
COPY --from=artifact /app/dist /usr/share/nginx/html
RUN chown nginx.nginx /usr/share/nginx/html/ -R
ENTRYPOINT ["nginx", "-g", "daemon off;"]

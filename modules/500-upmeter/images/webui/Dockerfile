FROM node:14-alpine3.12 as builder
# dependencies for favicon generator
RUN apk add python2 python3 vips make build-base
RUN mkdir -p /app
WORKDIR /app
COPY package.json /app/
COPY yarn.lock /app/
RUN yarn install
COPY . /app
RUN yarn run build

FROM nginx:1.15-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
RUN chown nginx.nginx /usr/share/nginx/html/ -R

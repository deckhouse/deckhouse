FROM node:14-alpine3.12 as builder
# dependencies for favicon generator
RUN apk add python3 vips make build-base
RUN mkdir -p /app
WORKDIR /app
COPY package*.json /app/
RUN npm ci
COPY . /app
RUN npm run build

FROM nginx:1.15-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
RUN chown nginx.nginx /usr/share/nginx/html/ -R

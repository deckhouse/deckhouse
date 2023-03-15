ARG BASE_NGINX_ALPINE
ARG BASE_NODE_16_ALPINE
FROM $BASE_NODE_16_ALPINE as artifact
RUN mkdir -p /app
WORKDIR /app
COPY package*.json /app/
RUN npm ci --ignore-scripts
COPY . /app
RUN NODE_ENV=production npm run build-only

FROM $BASE_NGINX_ALPINE
COPY --from=artifact /app/dist /app/dist

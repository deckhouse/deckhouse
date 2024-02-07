# Upmeter status page

## What is it

Upmeter is a Kubernetes statuspage. This project is the webpage that shows cluster status in a compact way.

## Development

This project is based on the template for [Svelte](https://svelte.dev) apps. It lives
at https://github.com/sveltejs/template.

Start the dev mode

```shell
$ npm i
$ npm run dev
```

Vite is configured to proxy API calls `/public/api/*` to localhost:8091. So, one can port-forward via SSH to the
cluster of choice. For example,

```shell
$ ssh <CLUSTER_NODE> -L8091:localhost:8091
$ kubectl -n d8-upmeter port-forward upmeter-0 8091:8091
```

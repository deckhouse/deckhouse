# Upmeter status page

## Development

Start the dev mode

```shell
$ npm i
$ npm run dev
```

Rollup is configured to proxy API calls `/public/api/*` to localhost:8091. So, one can port-forward via SSH to the
cluster of choice. For example,

```shell
$ ssh <CLUSTER_NODE> -L8091:localhost:8091
$ kubectl -n d8-upmeter port-forward upmeter-0 8091:8091
```

## It is Svelte

This project is based on the template for [Svelte](https://svelte.dev) apps. It lives
at https://github.com/sveltejs/template.

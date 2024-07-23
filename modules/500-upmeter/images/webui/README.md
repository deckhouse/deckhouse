# Upmeter web UI

## What is it?

Upmeter is a Kubernetes statuspage. This project it the webapp that shows probe statuses.

## Development

The SPA is written in React and d3.

```
$ yarn install
$ yarn run start:dev
```

On success, it prints "Compiled successfully". The page will be served on http://localhost:4800/ by default.

To run upmeter backend, one needs Kubernetes cluster with CRDs. It might be easier to connect to existing cluster with
upmeter

For example:

```
$ ssh <CLUSTER_NODE> -L8091:localhost:8091
$ sudo -i
# kubectl -n d8-upmeter port-forward upmeter-0 8091:8091
```

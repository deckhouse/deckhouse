
# site backend

HTTP-server to make some logic for website

## GET `/status`

Get status info (JSON).

- 'status' — `ok` or `error`.
- 'msg' — Empty if `status` is `ok`, otherwise contains text representation of the error.
- `rootVersion` — Version to show as main. E.g. - `v1.2.4+fix18`.
- `rootVersionURL` — URL location for RootVersion. E.g. - `v1.2.4-plus-fix18`.

## How to debug

There is the `docs/site/werf-debug.yaml` file to compile and the `docs/site/docker-compose-debug.yml` file to run the backend with [delve](https://github.com/go-delve/delve) debugger.

Run from the docs/site folder of the project (or run docs/site/backend/debug.sh):
```shell
werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'
```

Connect to localhost:2345

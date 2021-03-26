
# site backend

HTTP-server to make some logic for website

## GET `/status`

Get status info (JSON).

- 'status' — `ok` or `error`.
- 'msg' — Empty if `status` is `ok`, otherwise contains text representation of the error.
- `rootVersion` — Version to show as main. E.g. - `v1.2.4+fix18`.
- `rootVersionURL` — URL location for RootVersion. E.g. - `v1.2.4-plus-fix18`.

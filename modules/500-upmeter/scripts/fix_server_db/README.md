# Fix upmeter migration

## When to use it

Use it when a migration has stuck. Or when the disk is full.

For example, a migration cannot run (the migration number is not necessarily #4):

```bash
# kubectl -n d8-upmeter logs upmeter-0 upmeter
...
time="2021-05-20T10:22:50Z" level=fatal msg="cannot start server: database not connected: cannot migrate database: cannot migrate: Dirty database version 4. Fix and force version."
```

## What to do

1. Migrate
2. Optionally observe the state

### 1. Migrate

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- /modules/500-upmeter/scripts/fix_server_db/migrate.sh
```

### 2. Optionally observe the state

```shell
while true; do kubectl logs -f -n d8-upmeter upmeter-0 -c upmeter || sleep 2; done
watch kubectl -n d8-upmeter get po -l app=upmeter -o wide
```

`tmux -CC` and iTerm2 are your friends :-)

## How it helps

The fix is based on vacuuming sqlite file since it has too much unneccessary
data and prevents migrations from running. ON the migrations, the database file
doubles in size. Given PVC size is 1G, and the DB size is 600M, migration cannot
run.

We can copy the DB to /tmp, delete unnecessary data and shrink the file. In
SQLite it is called "VACUUM", hence the name of this dir.

```bash
$ tree
.
├── README.md
├── migrate.sh   # migration: main runner
└── __pod.sh     # migration: heavy-lifting in the Pod
```

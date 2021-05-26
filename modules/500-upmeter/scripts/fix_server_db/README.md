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

1. Copy migration scripts to a master node
2. Migrate
3. Optionally observe the state

### 1. Copy migration scripts to a master node

Copy scripts from `./vacuum` dir to a node, and connect via ssh there. The dir
will be copied to `/home/flant/$USER/vacuum`.

Use `deploy.sh` to do it in a single command:

```shell
$ ./deploy <MASTER_NODE_FROM_SSH_CONFIG>
```

### 2. Migrate

On the node, navigate to the scripts dir and authenticate as root:

```shell
you@node ~ $ cd vacuum
you@node ~/vacuum $ sudo su    # note no dash "-" here
```

Run the migration.

```shell
root@node ~/vacuum # ./migrate.sh
```

### 3. Optionally observe the state

Optionally track state with *o*-scripts which are observer helpers:

```shell
root@node ~/vacuum # ./o-logs.sh
root@node ~/vacuum # ./o-status.sh
```

`tmux -CC` and iTerm2 are your friends :-)

## Kubectl Context

If there are multiple kubectl contexts, pass the name of the desired context as the first argument to any script you use on node. It will be passed to the `--context` option of `kubectl`.

For example, to use context "dev"

```shell
# kubectl config get-contexts
CURRENT   NAME      CLUSTER   AUTHINFO   NAMESPACE
          prod      prod      prod
*         dev       dev       dev
```

```shell
root@node ~/vacuum # ./migrate.sh dev
...
root@node ~/vacuum # ./o-logs.sh dev
...
root@node ~/vacuum # ./o-status.sh dev
```

## How it helps

The fix is based on vacuuming sqlite file since it has too much unneccessary
data and prevents migrations from runnung. ON the migrations, the database file
doubles in size. Given PVC size is 1G, and the DB size is 600M, migration cannot
run.

We can copy the DB to /tmp, delete unnecessary data and shrink the file. In
SQLite it is called "VACUUM", hence the name of this dir.


```bash
$ tree
.
├── README.md
├── deploy.sh
└── vacuum
    ├── o-logs.sh    # tracking: see pod logs
    ├── o-status.sh  # tracking: see pod status
    ├── migrate.sh   # migration: main runner
    └── __pod.sh     # migration: heavy-lifting in the pod
```
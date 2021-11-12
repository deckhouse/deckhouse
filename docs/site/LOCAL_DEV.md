# How to start documentation container locally?

Don't forget to clone repo firstly.

1. Free 80 port to bind

3. Open console and start documentation container 
```shell
cd docs/documentation
source $(trdl use werf 1.2 ea)
docker network create deckhouse
werf compose up
```
3. Open separate console and start site container 
```shell
cd docs/site
source $(trdl use werf 1.2 ea)
werf compose up
```

## How to debug

There is the `docs/site/werf-debug.yaml` file to compile and the `docs/site/docker-compose-debug.yml` file to run the backend with [delve](https://github.com/go-delve/delve) debugger.

Run from the docs/site folder of the project (or run docs/site/backend/debug.sh):
```shell
werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'
```

Connect to localhost:2345

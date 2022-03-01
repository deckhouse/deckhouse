# How to start documentation container locally?

Don't forget to clone repo firstly.

Free 80 port to bind.

Open console and start documentation container:
```shell
source $(trdl use werf 1.2 ea)
cd docs/documentation
docker network create deckhouse
export BASE_NGINX_ALPINE=nginx:1.15.12-alpine@sha256:57a226fb6ab6823027c0704a9346a890ffb0cacde06bc19bbc234c8720673555
export BASE_ALPINE=alpine:3.12.1@sha256:c0e9560cda118f9ec63ddefb4a173a2b2a0347082d7dff7dc14272e7841a5b5a
export BASE_GOLANG_16_ALPINE=golang:1.16.3-alpine3.12@sha256:371dc6bf7e0c7ce112a29341b000c40d840aef1dbb4fdcb3ae5c0597e28f3061
export BASE_JEKYLL=jekyll/jekyll:3.8@sha256:9521c8aae4739fcbc7137ead19f91841b833d671542f13e91ca40280e88d6e34 
werf compose up --follow --docker-compose-command-options='-d'
```

Open separate console and start site container:
```shell
source $(trdl use werf 1.2 ea)
cd docs/site
export BASE_NGINX_ALPINE=nginx:1.15.12-alpine@sha256:57a226fb6ab6823027c0704a9346a890ffb0cacde06bc19bbc234c8720673555
export BASE_ALPINE=alpine:3.12.1@sha256:c0e9560cda118f9ec63ddefb4a173a2b2a0347082d7dff7dc14272e7841a5b5a
export BASE_GOLANG_16_ALPINE=golang:1.16.3-alpine3.12@sha256:371dc6bf7e0c7ce112a29341b000c40d840aef1dbb4fdcb3ae5c0597e28f3061
export BASE_JEKYLL=jekyll/jekyll:3.8@sha256:9521c8aae4739fcbc7137ead19f91841b833d671542f13e91ca40280e88d6e34 
werf compose up --follow --docker-compose-command-options='-d'
```

Open http://localhost.

Don't forget to stop documentation and site containers by running:
```shell
werf compose down
``` 

## How to debug

There is the `docs/site/werf-debug.yaml` file to compile and the `docs/site/docker-compose-debug.yml` file to run the backend with [delve](https://github.com/go-delve/delve) debugger.

Run from the docs/site folder of the project (or run docs/site/backend/debug.sh):
```shell
werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'
```

Connect to localhost:2345

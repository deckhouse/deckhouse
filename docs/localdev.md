# How to start documentation containers locally?

0. Free 80 port to bind (stop other services)

1. Create network if needed

```shell
docker network create deckhouse
```

2. Open separate console and start documentation container
```shell
cd docs/documentation
source $(multiwerf use 1.2 alpha --as-file)
werf compose up --follow --docker-compose-command-options='-d'
```

3. Open separate console and start main site containers
```shell
cd docs/site
source $(multiwerf use 1.2 alpha --as-file)
werf compose up --follow --docker-compose-command-options='-d'
```

4. Open localhost
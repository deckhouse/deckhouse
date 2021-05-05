
## How to get right cross link

TLDR:
- For link from `010-one` module overview to `020-two` module configuration page (...module/020-two/configuration.html) use one of the following link in the modules/010-one/README.md file:
  - `[link](../020-two/configuration.html)`
  - `[link]({{ "/module/020-two/configuration.html" | true_relative_url }})`
- In common, the following method will work for any link.
  - `[link]({{ "DST_LINK" | true_relative_url }})`, where `DST_LINK` - is path to the rendered file with URL relative to documentation version (right after version part of the URL). E.g. for `some.domain/en/documentation/v1/modules/040-control-plane-manager/usage.html`, `DST_LINK` is `/modules/040-control-plane-manager/usage.html` (but not USAGE.md).

## How to start documentation containers locally?

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
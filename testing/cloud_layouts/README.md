# Что-то пошло не так и job не завершился

Пример для Yandex.Cloud, но подходит для всех нештатных ситуаций.

docker run --entrypoint /bin/bash -ti -v /deckhouse:/deckhouse -v "$(pwd)/testing/cloud_layouts:/deckhouse/testing/cloud_layouts" -v "$(pwd)/layouts-tests-tmp:/tmp" -w /deckhouse registry.flant.com/sys/antiopa/dev/install:master

candictl destroy --ssh-agent-private-keys testing/cloud_layouts/Yandex.Cloud/WithoutNAT/sshkey --ssh-user ubuntu --ssh-host 178.154.227.70 --yes-i-am-sane-and-i-understand-what-i-am-doing

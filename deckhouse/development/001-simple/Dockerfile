FROM flant/addon-operator:latest-alpine3.9

RUN mkdir /deckhouse

# deckhouse binary
COPY deckhouse /deckhouse/

# copy libraries
COPY shell_lib.sh /deckhouse/
COPY shell_lib /deckhouse/shell_lib
COPY jq_lib /deckhouse/jq_lib
COPY helm_lib /deckhouse/helm_lib

# add hooks and modules files
COPY modules /deckhouse/modules
COPY global-hooks /deckhouse/global-hooks


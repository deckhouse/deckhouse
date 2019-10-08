FROM flant/addon-operator:latest-alpine3.9

RUN mkdir /antiopa

# antiopa binary
COPY antiopa /antiopa/

# copy libraries
COPY shell_lib.sh /antiopa/
COPY shell_lib /antiopa/shell_lib
COPY jq_lib /antiopa/jq_lib
COPY helm_lib /antiopa/helm_lib

# add hooks and modules files
COPY modules /antiopa/modules
COPY global-hooks /antiopa/global-hooks


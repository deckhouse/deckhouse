#!/bin/bash

# Copyright 2024 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

EXT_MODULES_DIR=$PWD/external_modules
BUILDER_TEMPLATE_PATH=$PWD/docs/site/backends/docs-builder-template
CONTENT_PATH=$BUILDER_TEMPLATE_PATH/content/modules
CRDS_PATH=$BUILDER_TEMPLATE_PATH/data/modules
MODULES=(
    "https://github.com/deckhouse/sds-replicated-volume"
    "https://github.com/deckhouse/sds-node-configurator"
)

function create_ext_modules_path () {
    test -e $EXT_MODULES_DIR || mkdir $EXT_MODULES_DIR
}

function clean () {
    rm -rf $EXT_MODULES_DIR
}

function get_modules () {
    for ix in ${!MODULES[*]}
    do
        unset MODULE_NAME
        MODULE_NAME=$(echo ${MODULES[$ix]} | sed 's/.*\///')
        echo "Getting module $MODULE_NAME"
        module_path=$(echo $EXT_MODULES_DIR/$MODULE_NAME)
        test -e $module_path || mkdir $CONTENT_PATH/$MODULE_NAME
        test -e $module_path || git clone ${MODULES[$ix]} $module_path
    done
}

function prepare_templates () {
    for ix in ${!MODULES[*]}
    do
        unset MODULE_NAME
        MODULE_NAME=$(echo ${MODULES[$ix]} | sed 's/.*\///')
        echo "Preparing docs of module $MODULE_NAME..."
        module_path=$(echo $EXT_MODULES_DIR/$MODULE_NAME)
        module_content_path=$(echo "$module_path/docs")
        template_content_path=$(echo "$CONTENT_PATH/$MODULE_NAME/alpha")
        test -e $template_content_path || mkdir -p $template_content_path
        cp -r $module_content_path/*.md $template_content_path
        find $template_content_path -name '*_RU.md' | sed 'p;s:_RU:.ru:g' | xargs -n2 mv

        echo "Preparing CRDs of module $MODULE_NAME..."
        template_crds_path=$(echo "$CRDS_PATH/$MODULE_NAME/alpha/crds")
        module_crds_path=$(echo "$module_path/crds")
        test -e $template_crds_path || mkdir -p $template_crds_path
        cp -r $module_crds_path/*.yaml $template_crds_path

        echo "Preparing OpenAPI of module $MODULE_NAME..."
        template_openapi_path=$(echo "$CRDS_PATH/$MODULE_NAME/alpha/openapi")
        module_openapi_path=$(echo "$module_path/openapi")
        test -e $template_openapi_path || mkdir -p $template_openapi_path
        cp -r $module_openapi_path/*.yaml $template_openapi_path
    done
}

function prepare_channels() {
    rm $CRDS_PATH/channels.yaml
    touch $CRDS_PATH/channels.yaml
    for ix in ${!MODULES[*]}
    do
        unset MODULE_NAME
        MODULE_NAME=$(echo ${MODULES[$ix]} | sed 's/.*\///')
        echo "Preparing channels.yaml for $MODULE_NAME..."
        echo "$MODULE_NAME:" >> $CRDS_PATH/channels.yaml
        echo "  channels:" >> $CRDS_PATH/channels.yaml
        echo "    alpha:" >> $CRDS_PATH/channels.yaml
        echo "      version: v1.1" >> $CRDS_PATH/channels.yaml
    done
}

function run_hugo() {
    docker run --rm -p 1313:1313 -v $BUILDER_TEMPLATE_PATH:/src klakegg/hugo:0.111.3-ext-alpine server --renderToDisk --disableFastRender --environment production
}

if [ "$#" -eq 0 ]; then
    create_ext_modules_path
    get_modules
    prepare_templates
    prepare_channels
else
    if [ "$1" = "clean" ]; then
        echo "Cleaning $EXT_MODULES_DIR..."
        rm -rf $EXT_MODULES_DIR
    elif [ "$1" = "run" ]; then
        run_hugo
    fi
fi

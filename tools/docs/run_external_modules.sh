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

function create_ext_modules_path () {
    test -e $EXT_MODULES_DIR || mkdir $EXT_MODULES_DIR
}

function clean () {
    rm -rf $EXT_MODULES_DIR
}

function prepare_template () {
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
}

function prepare_channels() {
    rm $CRDS_PATH/channels.yaml
    touch $CRDS_PATH/channels.yaml
    echo "Preparing channels.yaml for $MODULE_NAME..."
    echo "$MODULE_NAME:" >> $CRDS_PATH/channels.yaml
    echo "  channels:" >> $CRDS_PATH/channels.yaml
    echo "    alpha:" >> $CRDS_PATH/channels.yaml
    echo "      version: v1.1" >> $CRDS_PATH/channels.yaml
}

function run_hugo() {
    docker run --rm -p 1313:1313 -v $BUILDER_TEMPLATE_PATH:/src -w /src cibuilds/hugo:0.150.1 hugo server --disableFastRender --environment production
}

if [ "$1" = "clean" ]; then
    echo "Cleaning $EXT_MODULES_DIR..."
    rm -rf $EXT_MODULES_DIR
elif [ "$1" = "run" ]; then
    run_hugo
else
    unset MODULE_NAME
    repo_name=$(echo $1 | sed 's/.*\///')
    MODULE_NAME="${repo_name%.*}"

    create_ext_modules_path

    echo "Getting module $MODULE_NAME"
    module_path=$(echo $EXT_MODULES_DIR/$MODULE_NAME)
    test -e $module_path || mkdir $CONTENT_PATH/$MODULE_NAME
    if [ $# -lt 2 ]; then
      test -e $module_path || git clone $1 $module_path
    else
      test -e $module_path || git clone --branch $2 --single-branch $1 $module_path
    fi

    prepare_template
    prepare_channels
    clean
fi

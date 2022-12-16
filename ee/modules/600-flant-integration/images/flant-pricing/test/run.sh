#!/bin/bash
#
# Copyright 2022 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
export BINDING_CONTEXT_PATH="node_metrics__input_binding_context.json"
export METRICS_PATH="node_metrics__metrics"
export METRICS_PATH_EXPECTED="node_metrics__metrics_expected"
touch $METRICS_PATH
python "node_metrics_test.py"
rm $METRICS_PATH

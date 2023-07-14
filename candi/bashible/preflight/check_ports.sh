#!/usr/bin/env bash
{{- /*
# Copyright 2023 Flant JSC
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
*/}}
function check_port() {
    nc -z 127.0.0.1 $1 > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo -n "it is already open "
        return 1
    fi

    nc -l $1 > /dev/null 2>&1 &
    local ncpid=$!
    sleep 0.1

    nc -z 127.0.0.1 $1 > /dev/null 2>&1
    local exit_code=$?

    if ps -p $ncpid > /dev/null
    then
        kill -9 $ncpid
    fi

    return $exit_code
}

for port in 6443 2379 2380
do
    echo -n "Check port $port "
    check_port $port
    if [ $? -ne 0 ]; then
        echo "FAIL"
        exit 1
    fi
    echo "SUCCESS"
done

exit 0

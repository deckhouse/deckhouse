#!/usr/bin/env bash

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

#!/bin/bash

# run from the docs/site folder of the project

source <(multiwerf use 1.2 alpha)
werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d' --docker-compose-options='-f docker-compose-debug.yml'

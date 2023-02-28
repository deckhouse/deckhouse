# Upmeter

## What is it?

Upmeter is a Kubernetes statuspage.

This project is the backend. The resulting docker image is used as the server and agents. Agents probe kubernetes and custom
features within the cluster. The server collects and serves probe statuses to 'webui' and 'status' web apps.

## Package name

The package is named `d8.io/upmeter` to distinguish its name from Go standard library packages.

## Development

Use makefile.

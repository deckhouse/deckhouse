# Workflows for development branches

Development branches are all branches not matched to patterns:

- 'main'
- 'master'
- 'release-*'
- 'alpha'
- 'beta'
- 'early-access'
- 'stable'
- 'rock-solid'
- 'changelog/*'

Each pushed commit to development branch starts several workflows:

## Build and test

This workflow checks generated sources, builds images, runs different tests

## Validation

Validates changes in source files:

- the presence of license headers
- simultaneous changes in English and Russian documentation
- accidental cyrillic letters in non-documention files

## e2e

Use 'e2e/run' labels to activate e2e test for particular provider.

## Web deploy

Use 'deploy/web' labels to deploy site and documentation images to 'test' or 'stage' environment.

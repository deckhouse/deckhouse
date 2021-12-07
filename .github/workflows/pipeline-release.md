# Workflows for default branch and tags

Pull request has 'Checks' widget with the list of running workflow. Actions for default branch and tag have no such widget. A separate issue is used to check workflow statuses and to activate actions via labels. This issue caled 'release issue'.

Milestone should be created first to emerge the 'release issue' for planned tag.

Each commit to default branch will start 'Build and test' workflow similar to one for development branches. After all images are built, labels can be used to start other workflows.

## Build and test

This workflow checks generated sources, builds images, runs different tests and deploys site and documentation.

Push to default branch:
- site is deployed to production environment
- documentation is deployed to production environment as 'latest' version.

Push tag:
- documentation is deployed to stage and production environments.
- tag is used as version name.

A comment is created on start, each job updates this comment, and final status is reported on workflow finish.

## e2e

Use 'e2e/run' labels to activate e2e test for particular provider.

## Deploy release

Use 'deploy/deckhouse' labels to activate deploy release images to choosen channel.

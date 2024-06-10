---
title: "Build and publish"
permalink: en/module-development/build/
---

Deckhouse Kubernetes Platform (DKP) uses container registry to pull a module and update it. The container registry stores module artifacts. Module artifacts are created when a module is built and can then be uploaded (published) to the registry.

## Types of module artifacts

Module builds create three types of artifacts that are pushed to the container registry:
- **Images of application containers**. The build rules and source code for these images are stored in the [images](../structure/#images) subdirectory of the _application name_ directory. The built images are then specified in the templates and run in the cluster. Images are tagged with the [content-based tags](https://werf.io/documentation/v1.2/usage/build/process.html#tagging-images). Note that the [lib-helm](https://github.com/deckhouse/lib-helm) library must be enabled to use them in the templates.
- **Module image**. The module assembly rules are in the `werf.yaml` file in the module directory. The [semantic versioning](https://semver.org/) is used as image tags.
- **Release**. Module version artifact. Based on the release data, DKP decides whether to update a module in the cluster. Releases have two types of tags: a [semantic versioning](https://semver.org/) tag (just like with the module image) and a tag that matches the release channel (e.g., `alpha`, `beta`, etc.). [In the module template](https://github.com/deckhouse/modules-template/) there is an example of workfklow for GitHub Actions, where the release is created automatically during building.

## Building module artifacts and publishing them to the container registry

We suggest using the pre-compiled [GitHub Actions](https://github.com/deckhouse/modules-actions) to build module artifacts and upload them to the container registry as part of the CI/CD process.

[The module template repository](https://github.com/deckhouse/modules-template/) provides an example module. It contains a basic workflow for GitHub Actions and uses [GitHub Packages](https://github.com/features/packages) (ghcr.io) as the container registry. The example workflow follows the logic outlined below:

- Build module artifacts when changes are made to the PR and when changes are merged into the main branch.
- Build module artifacts from tags using the production container registry.
- Publish module to GitHub Packages container registry to the selected [stability channel](../versioning/#stability-channels) from a tag.

Module artifacts will be uploaded to `ghcr.io/<OWNER>/modules/`, which will serve as [module source](../../cr.html#modulesource).

For the module workflow to run smoothly, adjust the properties of your project on GitHub as follows:
- Open the _Settings -> Actions -> General_ page.
- Enable the _Read and write permissions_ parameter in the _Workflow permissions_ section.

You can also modify the workflow, use your own container registry, and design a more complex build and publish process (e.g., use separate container registries for development and production).

{% alert level="info" %}
You can build module artifacts locally using [werf](https://werf.io/) (this may come in handy, for example, [when debugging](../development/)).

You can also set up a custom module artifact build and publish processes for your CI/CD system similar to the workflow for GitHub Actions provided in the module template. However, this may require an in-depth understanding of the module build and publish processes. Please contact [the community](/community/) should you have any questions or issues.
{% endalert %}

Here is a general scenario for using the workflow provided [in the module template](https://github.com/deckhouse/modules-template/):
1. Publish the changes to the module code in the project branch on GitHub. This will trigger the module artifacts to be built and published to the container registry.
1. Create a new module release or add a tag in the [semantic versioning](https://semver.org/lang/ru/) format to the target commit.
1. Go to the _Actions_ section of the module repository on GitHub and select _Deploy_ in the workflow list on the left.
1. On the right side of the page, click on the _Run workflow_ drop-down list, select the desired release channel, and enter the target tag in the tag input box. Click the _Run workflow_ button.
1. Once the workflow is successfully executed, a new version of the module will be added to the container registry. The published version of the module can then be [used in the cluster](../run/).

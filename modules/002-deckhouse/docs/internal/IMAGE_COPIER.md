# Image Copier

As you know, there are several Deckhuse editions: CE, EE, and FE.
Now, the customer may stop using our services at some point.
In this case, it makes sense to switch the customer to use the CE edition. However, they might depend
on some features of the EE edition (e.g., their cluster is deployed on OpenStack).
If you switch the customer to the CE edition in this case, the cluster will fail to operate.
Also, once the service is complete, we no longer want to deliver Deckhouse updates to this customer.
So we have to push ALL our images to the customer registry and switch Deckhouse to use that registry.

We have created a script to re-push the images and a corresponding hook to simplify this task.

## How does it work?

You'll need to create `images-copier-config` Secret  in the `d8-system` Namespace
that contains credentials to the target registry and the resulting address (with a TAG) of the `deckhouse` Deployment image.

The hook adds the current Deckhouse credentials and a list of all module images `/deckhouse/modules/images_tags.json` to the Secret.

This list gets generated at build time and inserted into the Deckhouse image.
The hook starts a Job for the image copier script and mounts the Secret into a Pod container.

The hook then deletes the Secret, Job, and Pod if copying is successful.

## General sequence of actions

### Copying Images

- Run [the script](../../images/images-copier/generate-copier-secret.sh) and specify target registry credentials.
  
  Example:
  `REPO_USER="u" REPO_PASSWORD='Pass"Word' IMAGE_WITH_TAG="client.registry/repo/deckhouse:test" ./generate-copier-secret.sh`

  The tag is MANDATORY!!! The tag can basically be anything, e.g., `rock-solid-1.24.7`.
- STDOUT will display the contents of the Secret to be added to the cluster  
  and two commands for switching the registry after copying.
- Add the Secret: `kubectl create -f - <<EOF ...`
- The corresponding Job should now be available in the cluster: `kubectl -n d8-system get job copy-images`
- Wait for copying to finish.
- If copying is successful, the Job, Pod, and Secret will be deleted from the cluster, and the following message will be written to the Deckhouse log:
  `Image copier ran successfully. Cleanup`

  Here is the command to confirm that the process was successful: `kubectl logs -n d8-system deployments/deckhouse | grep "Image copier ran successfully"`
- If copying fails, the Job, Pod, and Secret will not be deleted, while the following message will be added to the Deckhouse log:
  `Image copier was failed. See logs into image copier job pod for additional information`

  Here is the command to confirm that the process wasn't successful: `kubectl logs -n d8-system deployments/deckhouse | grep "Image copier was failed"`
  
  In this case, we recommend searching logs, e.g., using the following command:
  `kubectl -n d8-system logs jobs/copy-images`

  You can restart the process by adding/modifying the annotations in the `d8-system/images-copier-config` Secret.

Caution! If you delete Secret, the Job and Pod will be deleted as well regardless of the status of the Job.

### Switching the repository

Execute commands another commands from `generate-copier-secret.sh` output:

* Change Deckhouse Deployment image.
* Change `d8-system/deckhouse-registry secret.
* Wait for Deckhouse pod ready (restart pod if it is in ImagePullBackoff state).
* Check bashible on master node restart correctly.
* If you use the istio module, it is recommended to restart all the application pods with istio sidecar.
* Check if there are Pods with original registry:

  ```shell
  kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io")))) | .metadata.namespace + "\t" + .metadata.name' -r
  ```

Or use this [instruction](https://deckhouse.io/documentation/v1/deckhouse-faq.html#how-do-i-switch-a-running-deckhouse-cluster-to-use-a-third-party-registry)

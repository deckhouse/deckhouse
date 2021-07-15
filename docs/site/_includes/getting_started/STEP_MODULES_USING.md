The Deckhouse Platform module system allows you to add modules to the cluster and delete them on the fly. All you need to do is edit the cluster config — Deckhouse Platform will apply all the necessary changes automatically.

Let's, for example, add the [user-authn](/en/documentation/v1/modules/150-user-authn/) module:

-  Open the Deckhouse configuration:
   ```yaml
kubectl -n d8-system edit cm/deckhouse
```
-  Find the `data` section and add enable the module there:
   ```yaml
data:
  userAuthnEnabled: "true"
```
-  Save the configuration file. At this point, Deckhouse notices the changes and installs the module automatically.

To edit the module settings, repeat the steps (make changes to the configuration and save them). The changes will be applied automatically.

To disable the module, set the parameter to `false`.

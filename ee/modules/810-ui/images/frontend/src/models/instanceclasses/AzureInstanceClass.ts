// @ts-ignore
import InstanceClassBase from "./InstanceClassBase";

class AzureInstanceClass extends InstanceClassBase {
  public static klassName: string = "AzureInstanceClass";

  public kind: string = "AzureInstanceClass";
}

AzureInstanceClass.resourceBaseUrl = `k8s/deckhouse.io/azureinstanceclasses`;

// @ts-ignore
AzureInstanceClass.setRoutes(AzureInstanceClass.resourceBaseUrl, {}, AzureInstanceClass.rawRoutes(), {
  queryCache: true,
  noQueryFilters: true,
});
// @ts-ignore
AzureInstanceClass.initSubscription("GroupResourceChannel", { groupResource: "azureinstanceclasses.deckhouse.io" });

export default AzureInstanceClass;

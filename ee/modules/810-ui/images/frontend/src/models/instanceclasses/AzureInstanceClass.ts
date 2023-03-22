// @ts-ignore
import InstanceClassBase from "./InstanceClassBase";

class AzureInstanceClass extends InstanceClassBase {
  public static klassName: string = "AzureInstanceClass";
}

AzureInstanceClass.resourceBaseUrl = `k8s/deckhouse.io/AzureInstanceClasses`;

// @ts-ignore
AzureInstanceClass.setRoutes(AzureInstanceClass.resourceBaseUrl, {}, AzureInstanceClass.rawRoutes(), {
  queryCache: true,
  noQueryFilters: true,
});
// @ts-ignore
AzureInstanceClass.initSubscription("GroupResourceChannel", { groupResource: "azureinsanceclasses.deckhouse.io" });

export default AzureInstanceClass;

// @ts-ignore
import InstanceClassBase from "./InstanceClassBase";

class VsphereInstanceClass extends InstanceClassBase {
  public static klassName: string = "VsphereInstanceClass";
  public kind: string = "VsphereInstanceClass";
}

VsphereInstanceClass.resourceBaseUrl = `k8s/deckhouse.io/vsphereinstanceclasses`;

// @ts-ignore
VsphereInstanceClass.setRoutes(VsphereInstanceClass.resourceBaseUrl, {}, VsphereInstanceClass.rawRoutes(), {
  queryCache: true,
  noQueryFilters: true,
});
// @ts-ignore
VsphereInstanceClass.initSubscription("GroupResourceChannel", { groupResource: "vsphereinstanceclasses.deckhouse.io" });

export default VsphereInstanceClass;

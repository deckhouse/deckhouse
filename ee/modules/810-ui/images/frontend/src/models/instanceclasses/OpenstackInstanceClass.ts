// @ts-ignore
import InstanceClassBase from "./InstanceClassBase";
import type { InstanceClassAttributes } from "./InstanceClassBase";

class OpenstackInstanceClass extends InstanceClassBase {
  public static klassName: string = "OpenstackInstanceClass";
  public kind: string = "OpenStackInstanceClass";

  public get diskSizeGb(): string | undefined {
    return this.spec?.rootDiskSize;
  }
}

OpenstackInstanceClass.resourceBaseUrl = `k8s/deckhouse.io/openstackinstanceclasses`;

// @ts-ignore
OpenstackInstanceClass.setRoutes(OpenstackInstanceClass.resourceBaseUrl, {}, OpenstackInstanceClass.rawRoutes(), {
  queryCache: true,
  noQueryFilters: true,
});
// @ts-ignore
OpenstackInstanceClass.initSubscription("GroupResourceChannel", { groupResource: "openstackinsanceclasses.deckhouse.io" });

export default OpenstackInstanceClass;

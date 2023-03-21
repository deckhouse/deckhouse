// @ts-ignore
import InstanceClassBase from "./InstanceClassBase";
import type { InstanceClassAttributes } from "./InstanceClassBase";

class OpenstackInstanceClass extends InstanceClassBase {
  constructor(attrs: InstanceClassAttributes) {
    super(attrs);

    this.klassName = "OpenstackInstanceClass";
  }

  public get diskSizeGb(): string | undefined {
    return this.spec?.rootDiskSizeGb;
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

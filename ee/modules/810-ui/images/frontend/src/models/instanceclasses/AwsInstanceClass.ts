// @ts-ignore
import InstanceClassBase from "./InstanceClassBase";
import type { InstanceClassAttributes } from "./InstanceClassBase";
import type { IBadge } from "@/types";

class AwsInstanceClass extends InstanceClassBase {
  constructor(attrs: InstanceClassAttributes) {
    super(attrs);

    this.klassName = "AwsInstanceClass";
  }

  public get badges(): IBadge[] {
    const badges: IBadge[] = super.badges;

    if (this.spec.spot) badges.push({ title: "Spot", type: "warning" });
    return badges;
  }
}

AwsInstanceClass.resourceBaseUrl = `k8s/deckhouse.io/awsinstanceclasses`;

// @ts-ignore
AwsInstanceClass.setRoutes(AwsInstanceClass.resourceBaseUrl, {}, AwsInstanceClass.rawRoutes(), {
  queryCache: true,
  noQueryFilters: true,
});
// @ts-ignore
AwsInstanceClass.initSubscription("GroupResourceChannel", { groupResource: "awsinsanceclasses.deckhouse.io" });

export default AwsInstanceClass;

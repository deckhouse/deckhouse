// @ts-ignore
import InstanceClassBase from "./InstanceClassBase";
import type { Badge } from "@/types";

class AwsInstanceClass extends InstanceClassBase {
  public static klassName: string = "AwsInstanceClass";

  public kind: string = "AwsInstanceClass";

  public get badges(): Badge[] {
    const badges: Badge[] = super.badges;

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
AwsInstanceClass.initSubscription("GroupResourceChannel", { groupResource: "awsinstanceclasses.deckhouse.io" });

export default AwsInstanceClass;

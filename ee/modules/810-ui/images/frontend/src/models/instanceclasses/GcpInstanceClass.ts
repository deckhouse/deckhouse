// @ts-ignore
import InstanceClassBase from "./InstanceClassBase";
import type { InstanceClassAttributes } from "./InstanceClassBase";
import type { IBadge } from "@/types";

class GcpInstanceClass extends InstanceClassBase {
  public static klassName: string = "GcpInstanceClass";

  public get badges(): IBadge[] {
    const badges: IBadge[] = super.badges;

    if (this.spec.preemptible) badges.push({ title: "Preemptible", type: "warning" });
    return badges;
  }
}

GcpInstanceClass.resourceBaseUrl = `k8s/deckhouse.io/GcpInstanceClasses`;

// @ts-ignore
GcpInstanceClass.setRoutes(GcpInstanceClass.resourceBaseUrl, {}, GcpInstanceClass.rawRoutes(), {
  queryCache: true,
  noQueryFilters: true,
});
// @ts-ignore
GcpInstanceClass.initSubscription("GroupResourceChannel", { groupResource: "gcpinsanceclasses.deckhouse.io" });

export default GcpInstanceClass;

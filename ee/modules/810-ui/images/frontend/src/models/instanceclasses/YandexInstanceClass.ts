// @ts-ignore
import InstanceClassBase from "./InstanceClassBase";

class YandexInstanceClass extends InstanceClassBase {
  public static klassName: string = "YandexInstanceClass";
  public kind: string = "YandexInstanceClass";
}

YandexInstanceClass.resourceBaseUrl = `k8s/deckhouse.io/yandexinstanceclasses`;

// @ts-ignore
YandexInstanceClass.setRoutes(YandexInstanceClass.resourceBaseUrl, {}, YandexInstanceClass.rawRoutes(), {
  queryCache: true,
  noQueryFilters: true,
});
// @ts-ignore
YandexInstanceClass.initSubscription("GroupResourceChannel", { groupResource: "yandexinstanceclasses.deckhouse.io" });

export default YandexInstanceClass;

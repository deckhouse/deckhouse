// @ts-ignore
import InstanceClassBase from "./InstanceClassBase";

class YandexInstanceClass extends InstanceClassBase {
  public static klassName: string = "YandexInstanceClass";
}

YandexInstanceClass.resourceBaseUrl = `k8s/deckhouse.io/YandexInstanceClasses`;

// @ts-ignore
YandexInstanceClass.setRoutes(YandexInstanceClass.resourceBaseUrl, {}, YandexInstanceClass.rawRoutes(), {
  queryCache: true,
  noQueryFilters: true,
});
// @ts-ignore
YandexInstanceClass.initSubscription("GroupResourceChannel", { groupResource: "yandexinsanceclasses.deckhouse.io" });

export default YandexInstanceClass;

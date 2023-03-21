// @ts-ignore
import NxnResourceWs from "@lib/nxn-common/models/NxnResourceWs";

interface DeckhouseReleaseMetadata {
  creationTimestamp: string;
  generation?: number;
  name?: string;
  resourceVersion?: number;
  uid?: string;
}

interface IDeckhouseReleaseStatus {
  approved: boolean;
  phase: string;
  message: string;
  transitionTime?: string;
}

interface DeckhouseReleaseAttributes {
  apiVersion: string;
  approved: boolean;
  kind: string;
  metadata: DeckhouseReleaseMetadata;
  spec: {
    [key: string]: string | object;
  };
  status: IDeckhouseReleaseStatus;
}

class DeckhouseRelease extends NxnResourceWs implements DeckhouseReleaseAttributes {
  public static ws_disconnected: boolean;
  public ws_disconnected?: boolean; // probably not needed, TODO: review necessity
  public klassName: string;
  public is_stale: boolean = false;

  public status: IDeckhouseReleaseStatus;
  public apiVersion: string;
  public approved: boolean;
  public metadata: DeckhouseReleaseMetadata;
  public kind: string;
  public spec: { [key: string]: string | object };

  constructor(attrs: DeckhouseReleaseAttributes) {
    super();
    this.apiVersion = attrs.apiVersion;
    this.approved = attrs.approved;
    this.metadata = attrs.metadata;
    this.kind = attrs.kind;
    this.spec = attrs.spec;
    this.status = attrs.status;
    this.klassName = "DeckhouseRelease";
  }

  public static toPrimaryKey(model: DeckhouseRelease): string | undefined {
    return model.metadata && model.metadata.uid;
  }

  public static toVersionKey(model: DeckhouseRelease): number | undefined {
    return model.metadata?.resourceVersion;
  }

  public static onWsDisconnect() {
    if (this.ws_disconnected) return;
    this.ws_disconnected = true;
    this.all().forEach((item: DeckhouseRelease) => {
      item.ws_disconnected = true;
    });
  }

  public static async query(params: object = {}): Promise<Array<DeckhouseRelease>> {
    return Promise.reject("DeckhouseRelease:NotImplemented");
  }

  public static async get(params: object = {}): Promise<DeckhouseRelease | string | null> {
    return Promise.reject("DeckhouseRelease:NotImplemented");
  }

  private static async update(params: object = {}): Promise<null> {
    return Promise.reject("DeckhouseRelease:NotImplemented");
  }

  public async approve(params: object = {}): Promise<null> {
    this.approved = true;
    const updateAttrs = (({ klassName, is_stale, ...o }) => o)(this);
    return this.constructor.update({ name: this.metadata.name }, updateAttrs);
  }
}

// var resourceBaseUrl = `${window.location.protocol}//:hostname/:api_path/deckhousereleases`;
const resourceBaseUrl = `k8s/deckhouse.io/deckhousereleases`;
DeckhouseRelease.setRoutes(
  resourceBaseUrl,
  {},
  {
    query:   { method: "GET", storeResponse: true, queryCache: true, format: "array", withCredentials: false },
    get:     { method: "GET", url: resourceBaseUrl + "/:name", storeResponse: false, withCredentials: false },
    update:  { method: "PUT", url: resourceBaseUrl + "/:name", withCredentials: false }
  },
  {
    queryCache: true,
    noQueryFilters: true
  }
);
DeckhouseRelease.initSubscription("GroupResourceChannel", { groupResource: "deckhousereleases.deckhouse.io" });

export default DeckhouseRelease;

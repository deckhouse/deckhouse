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

export interface DeckhouseReleaseChangelog {
  [key: string]: {
    fixes: {
      impact?: string;
      pull_request: string;
      summary: string;
    }[];
  };
}

interface DeckhouseReleaseSpec {
  version: string;
  changelogLink: string;
  changelog: DeckhouseReleaseChangelog;
  [key: string]: string | object;
}

interface DeckhouseReleaseAttributes {
  kind: string;
  apiVersion: string;
  approved: boolean;
  metadata: DeckhouseReleaseMetadata;
  spec: DeckhouseReleaseSpec;
  status?: IDeckhouseReleaseStatus;
}

class DeckhouseRelease extends NxnResourceWs<DeckhouseRelease> implements DeckhouseReleaseAttributes {
  public static ws_disconnected: boolean;
  public static klassName: string = "DeckhouseRelease";
  public is_stale: boolean = false;

  public apiVersion: string = "deckhouse.io/v1";
  public kind: string = "DeckhouseRelease";

  public status?: IDeckhouseReleaseStatus;
  public approved: boolean;
  public metadata: DeckhouseReleaseMetadata;
  public spec: DeckhouseReleaseSpec;

  constructor(attrs: DeckhouseReleaseAttributes) {
    super();
    this.kind = attrs.kind;
    this.apiVersion = attrs.apiVersion;

    this.approved = attrs.approved;
    this.metadata = attrs.metadata;
    this.spec = attrs.spec;
    this.status = attrs.status;
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
    const updateAttrs = (({ is_stale, ...o }) => o)(this);
    return this.constructor.update({ name: this.metadata.name }, updateAttrs);
  }
}

// var resourceBaseUrl = `${window.location.protocol}//:hostname/:api_path/deckhousereleases`;
const resourceBaseUrl = `k8s/deckhouse.io/deckhousereleases`;
DeckhouseRelease.setRoutes(
  resourceBaseUrl,
  {},
  {
    query: { method: "GET", storeResponse: true, queryCache: true, format: "array", withCredentials: false },
    get: { method: "GET", url: resourceBaseUrl + "/:name", storeResponse: false, withCredentials: false },
    update: { method: "PUT", url: resourceBaseUrl + "/:name", withCredentials: false },
  },
  {
    queryCache: true,
    noQueryFilters: true,
  }
);
DeckhouseRelease.initSubscription("GroupResourceChannel", { groupResource: "deckhousereleases.deckhouse.io" });

export default DeckhouseRelease;

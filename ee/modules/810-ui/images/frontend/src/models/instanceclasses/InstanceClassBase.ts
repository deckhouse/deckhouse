// @ts-ignore
import type { Badge } from "@/types";
import NxnResourceWs from "@lib/nxn-common/models/NxnResourceWs";
import Discovery, { type IInstanceTypeInfo } from "../Discovery";

interface IInstanceClassMetadata {
  name: string;
  creationTimestamp?: string;
  labels?: { [key: string]: string };
  annotations?: { [key: string]: string };
  uid?: string;
  resourceVersion?: string;
}

interface IInstanceClassSpec {
  [key: string]: any;
  runtimeOptions?: { [key: string]: any };
}

interface InstanceClassStatus {
  nodeGroupConsumers: string[];
}

export interface InstanceClassAttributes {
  apiVersion?: string;
  kind?: string;
  metadata?: IInstanceClassMetadata;
  spec?: IInstanceClassSpec;
  status?: InstanceClassStatus;
  isNew?: boolean;
}

abstract class InstanceClassBase extends NxnResourceWs {
  public static resourceBaseUrl: string;
  public static ws_disconnected: boolean;
  public static klassName: string;
  public ws_disconnected?: boolean; // probably not needed, TODO: review necessity
  public is_stale: boolean = false;
  public isNew?: boolean = false;
  public nodeGroupName?: string;

  public apiVersion: string = "deckhouse.io/v1";
  public kind: string = "InstanceClass"; // need to initialize in child class
  public metadata: IInstanceClassMetadata;
  public spec: IInstanceClassSpec;
  public status: InstanceClassStatus;

  constructor(attrs: InstanceClassAttributes) {
    super();
    this.apiVersion = attrs.apiVersion || this.apiVersion;
    this.kind = attrs.kind || this.kind;
    this.metadata = attrs.metadata || ({} as IInstanceClassMetadata);
    this.spec = attrs.spec || ({} as IInstanceClassSpec);
    this.status = attrs.status || ({} as InstanceClassStatus);

    this.isNew = attrs.isNew;
  }

  public static toPrimaryKey(model: InstanceClassBase): string | undefined {
    return model?.name;
  }

  public static toVersionKey(model: InstanceClassBase): string | undefined {
    return model.metadata?.creationTimestamp;
  }

  public async save(): Promise<InstanceClassBase | null> {
    const attrs = (({ is_stale, isNew, ...o }) => o)(this);
    if (this.isNew) {
      return this.constructor.create({}, attrs).then(() => {
        delete this.isNew;
      });
    } else {
      return this.constructor.update({ name: this.name }, attrs);
    }
  }

  public async delete(): Promise<InstanceClassBase | null> {
    return this.constructor.delete({ name: this.name }).then(() => {
      this.nxndbDestroy();
    });
  }

  public get name(): string | undefined {
    return this.metadata?.name;
  }

  public get creationTimestamp(): string {
    return this.metadata?.creationTimestamp || Date.now().toString();
  }

  public get instanceTypeInfo(): IInstanceTypeInfo {
    if (!this.spec?.instanceType) return {} as IInstanceTypeInfo;

    return Discovery.get().instanceTypeInfo(this.spec.instanceType);
  }

  public get badges(): Badge[] {
    const badges: Badge[] = [];
    return badges;
  }

  // public set instanceTypeInfo(val: object | undefined) {
  //   this._instanceTypeInfo = val;
  // }

  public static rawRoutes(): object {
    return {
      query: { method: "GET", storeResponse: true, queryCache: true, format: "array", withCredentials: false },
      get: { method: "GET", url: this.resourceBaseUrl + "/:name", storeResponse: true, withCredentials: false },
      create: { method: "POST", url: this.resourceBaseUrl, withCredentials: false },
      update: { method: "PUT", url: this.resourceBaseUrl + "/:name", withCredentials: false },
      delete: { method: "DELETE", url: this.resourceBaseUrl + "/:name", withCredentials: false },
    };
  }
}

// IMPLEMENT IN REAL CLASS:

// InstanceClassBase.resourceBaseUrl = `k8s/deckhouse.io/instanceclasses`;

// // @ts-ignore
// InstanceClassBase.setRoutes(
//   InstanceClassBase.resourceBaseUrl,
//   {},
//   InstanceClassBase.rawRoutes(),
//   { dynamic_cache: false }
// );
// @ts-ignore
// InstanceClassBase.initSubscription("GroupResourceChannel", { groupResource: "insanceclasses.deckhouse.io" });

export default InstanceClassBase;

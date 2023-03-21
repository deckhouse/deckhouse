import { objectAsArray } from "@/utils";
// @ts-ignore
import NxnResourceWs from "@lib/nxn-common/models/NxnResourceWs";
import InstanceClasses from "./instanceclasses";
import type { InstanceClassesTypes } from "./instanceclasses";

export interface IInstanceTypeInfo {
  InstanceType: string;
  VCPU: number;
  MemoryMb: number;
  GPU: number;
  Architecture: string;
}

interface ICloudProvider {
  name: string;
  configuration: {
    zones: string[];
  };
  knownInstanceTypes: { [key: string]: IInstanceTypeInfo };
}

class Discovery extends NxnResourceWs {
  public klassName: string;

  public kubernetesVersion: string;
  public cloudProvider: ICloudProvider;

  constructor(attrs: Discovery) {
    super();
    this.kubernetesVersion = attrs.kubernetesVersion;
    this.cloudProvider = attrs.cloudProvider;
    this.klassName = "Discovery";
  }

  public static get(): Discovery {
    return this.find("discovery");
  }

  public static toPrimaryKey(model: Discovery): string {
    return "discovery";
  }

  public static toVersionKey(model: Discovery): string {
    return model.kubernetesVersion;
  }

  public get instanceClassKlass(): (typeof InstanceClasses)[keyof typeof InstanceClasses] {
    return InstanceClasses[this.cloudProvider.name as keyof typeof InstanceClasses];
  }

  public get knownInstanceTypes(): IInstanceTypeInfo[] {
    return Object.values(this.cloudProvider.knownInstanceTypes);
  }

  public get availableZones(): string[] {
    return this.cloudProvider.configuration.zones;
  }

  public instanceTypeInfo(instanceType: string): IInstanceTypeInfo {
    return this.knownInstanceTypes.find((it) => it.InstanceType == instanceType) || ({} as IInstanceTypeInfo);
  }

  public static async load(params: object = {}): Promise<Discovery | null> {
    return Promise.reject("Discovery:NotImplemented");
  }
}

// @ts-ignore
Discovery.setRoutes(
  `discovery`,
  {},
  {
    load: { method: "GET", storeResponse: true, queryCache: true, withCredentials: false },
  },
  {
    queryCache: true,
    noQueryFilters: true,
  }
);
// @ts-ignore
Discovery.initSubscription("DiscoveryChannel");

export default Discovery;

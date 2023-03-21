import type { IBadge, IStatusCondition, ITaint, IUpdateWindow } from "@/types";
// @ts-ignore
import NxnResourceWs from "@lib/nxn-common/models/NxnResourceWs";

interface NodeGroupMetadata {
  creationTimestamp?: string;
  name: string;
  resourceVersion?: number;
  uid?: string;
  labels?: any;
  annotations?: any;
}

interface NodeGroupStatus {
  conditionSummary: {
    ready: string;
    statusMessage: string;
  };
  conditions: IStatusCondition[];
  nodes: number;
  ready: number;
  desired?: number;
  instances?: number;
  lastMachineFailures?: any[];
  max?: number;
  min?: number;
  standby?: number;
  upToDate?: number;
  error?: string;
  kubernetesVersion?: string;
}

interface NodeGroupAttributes {
  isNew?: boolean;
  apiVersion?: string;
  kind?: string;
  metadata?: NodeGroupMetadata;
  spec: NodeGroupSpec;
  status?: NodeGroupStatus;
}

interface DisruptionConfig {
  windows: IUpdateWindow[];
}

export const ApprovalModes = ["Manual", "Automatic", "RollingUpdate"] as const;

export const NodeTypes = ["CloudEphemeral", "CloudPermanent", "CloudStatic", "Static"] as const;

export type NodeTypesType = (typeof NodeTypes)[number];

export interface NodeGroupSpec {
  nodeType: NodeTypesType;
  nodeTemplate?: {
    labels?: { [key: string]: any };
    annotations?: { [key: string]: any };
    taints?: ITaint[];
  };
  cloudInstances?: {
    classReference: {
      kind: string;
      name: string;
    };
    priority: number;
    maxPerZone: number;
    minPerZone: number;
    maxUnavailablePerZone: number;
    maxSurgePerZone: number;
    standby: number;
    standbyHolder: {
      overprovisioningRate: number;
    };
    quickShutdown: boolean;
    zones: string[];
  };
  disruptions?: {
    approvalMode: (typeof ApprovalModes)[number];
    automatic?: DisruptionConfig & {
      drainBeforeApproval: boolean;
    };
    rollingUpdate?: DisruptionConfig;
  };
  cri?: {
    type: "Docker" | "Containerd" | "NotManaged";
    docker?: {
      maxConcurrentDownloads: number;
      manage: boolean;
    };
    containerd?: {
      maxConcurrentDownloads: number;
    };
  };
  operatingSystem?: {
    manageKernel: boolean;
  };
  kubelet?: {
    containerLogMaxFiles: number;
    containerLogMaxSize: string;
    maxPods: number;
    rootDir: string;
  };
  chaos?: {
    mode: "DrainAndDelete" | "Disabled";
    period?: string;
  };
  [key: string]: any;
}

class NodeGroup extends NxnResourceWs implements NodeGroupAttributes {
  public static ws_disconnected: boolean;
  public static klassName: string = "NodeGroup";
  public ws_disconnected?: boolean; // probably not needed, TODO: review necessity
  public is_stale: boolean = false;
  public isNew?: boolean = false;

  public apiVersion?: string;
  public kind?: string;
  public metadata?: NodeGroupMetadata;
  public spec: NodeGroupSpec;
  public status?: NodeGroupStatus;

  constructor(attrs: NodeGroupAttributes) {
    super();
    this.apiVersion = attrs.apiVersion;
    this.metadata = attrs.metadata;
    this.kind = attrs.kind;
    this.spec = attrs.spec;
    this.status = attrs.status;

    this.isNew = attrs.isNew;
  }

  public static toPrimaryKey(model: NodeGroup): string | undefined {
    return model.metadata && model.metadata.uid;
  }

  public static toVersionKey(model: NodeGroup): string | undefined {
    return model.metadata?.creationTimestamp;
  }

  public static onWsDisconnect() {
    if (this.ws_disconnected) return;
    this.ws_disconnected = true;
    this.all().forEach((item: NodeGroup) => {
      item.ws_disconnected = true;
    });
    console.log('this.$eventBus.emit("::wsDisconnected", "Incident");');
  }

  public static async query(params: object = {}): Promise<Array<NodeGroup>> {
    return Promise.reject("NodeGroup:NotImplemented");
  }

  public static async get(params: object = {}): Promise<NodeGroup | null> {
    return Promise.reject("NodeGroup:NotImplemented");
  }

  public async save(): Promise<NodeGroup | null> {
    const attrs = (({ is_stale, isNew, ...o }) => o)(this);
    if (this.isNew) {
      return this.constructor.create({}, attrs).then(() => {
        delete this.isNew;
      });
    } else {
      return this.constructor.update({ name: this.name }, attrs);
    }
  }

  public async delete(): Promise<void> {
    return this.constructor.delete({ name: this.metadata.name }).then(() => {
      this.nxndbDestroy();
    });
  }

  // Attributes

  public get name(): string | undefined {
    return this.metadata?.name;
  }

  // TODO: move to view?
  public get badges(): IBadge[] {
    const badges: IBadge[] = [];

    if (!this.status?.conditions?.length) return badges;

    for (const condition of this.status.conditions) {
      switch (condition.type) {
        case "Ready": {
          badges.push(condition.status == "True" ? { title: "Ready", type: "success" } : { title: "NotReady", type: "error" });
          break;
        }
        case "Updating": {
          if (condition.status == "True") badges.push({ title: "Обновляется", type: "info", loading: true });
          break;
        }
        case "WaitingForDisruptiveApproval": {
          if (condition.status == "True") badges.push({ title: "Ждёт ручного подтверждения", type: "warning" });
          break;
        }
        case "Scaling": {
          if (condition.status == "True") badges.push({ title: "Масштабируется", type: "info", loading: true });
          break;
        }
        case "Error": {
          if (condition.status == "True") badges.push({ title: "Ошибка", type: "error" });
          break;
        }
      }
    }
    return badges;
  }

  public get errorMessages(): string[] {
    let messages: string[] = [];
    const error_condition = this.status?.conditions?.find((c) => c.type == "Error" && c.status == "True");
    const disruption_condition = this.status?.conditions?.find((c) => c.type == "WaitingForDisruptiveApproval" && c.status == "True");
    if (error_condition?.message) {
      messages = messages.concat(error_condition.message.split(";"));
    }

    if (disruption_condition) {
      messages.push("Необходимо заапрувить узлы");
    }

    return messages;
  }

  public get cloudInstanceKind(): string | undefined {
    return this.spec.cloudInstances?.classReference?.name;
  }

  public get zones(): Array<string> {
    return this.spec.cloudInstances?.zones || [];
  }

  public set zones(newVal: Array<string>) {
    this.spec.cloudInstances!.zones = newVal.slice();
  }

  public get priority() {
    return this.spec.cloudInstances?.priority;
  }

  public get kubernetesVersion() {
    return this.status?.kubernetesVersion;
  }

  public get isAutoscalable(): boolean {
    return this.spec.nodeType == "CloudEphemeral";
  }
}

// var resourceBaseUrl = `${window.location.protocol}//:hostname/:api_path/nodegroups`;
const resourceBaseUrl = `k8s/deckhouse.io/nodegroups`;
NodeGroup.setRoutes(
  resourceBaseUrl,
  {},
  {
    query: { method: "GET", storeResponse: true, queryCache: true, format: "array", withCredentials: false },
    get: { method: "GET", url: resourceBaseUrl + "/:name", storeResponse: true, withCredentials: false },
    create: { method: "POST", url: resourceBaseUrl, withCredentials: false },
    update: { method: "PUT", url: resourceBaseUrl + "/:name", withCredentials: false },
    delete: { method: "DELETE", url: resourceBaseUrl + "/:name", withCredentials: false },
  },
  {
    queryCache: true,
    noQueryFilters: true,
  }
);
NodeGroup.initSubscription("GroupResourceChannel", { groupResource: "nodegroups.deckhouse.io" });

export default NodeGroup;

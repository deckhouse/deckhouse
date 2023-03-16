// @ts-ignore
import NxnResourceWs from "@lib/nxn-common/models/NxnResourceWs";

interface INodeMetadata {
  creationTimestamp: string;
  name?: string;
  resourceVersion?: number;
  uid?: string;
  labels: any;
  annotations: any;
}

interface INodeStatus {
  addresses: any[];
  allocatable: any;
  capacity: any;
  conditions: any[];
  daemonEndpoints: any;
  images: any[];
  nodeInfo: any;
  volumesAttached: any[];
  volumesInUse?: string[];
}

interface NodeAttributes {
  apiVersion: string;
  kind: string;
  metadata: INodeMetadata;
  spec: {
    [key: string]: any;
  };
  status: INodeStatus;
}

class Node extends NxnResourceWs implements NodeAttributes {
  public static ws_disconnected: boolean;
  public ws_disconnected?: boolean; // probably not needed, TODO: review necessity
  public klassName: string;
  public is_stale: boolean = false;

  public status: INodeStatus;
  public apiVersion: string;
  public metadata: INodeMetadata;
  public kind: string;
  public spec: { [key: string]: any };

  constructor(attrs: NodeAttributes) {
    super();
    this.apiVersion = attrs.apiVersion;
    this.metadata = attrs.metadata;
    this.kind = attrs.kind;
    this.spec = attrs.spec;
    this.status = attrs.status;
    this.klassName = "Node";
  }

  public static toPrimaryKey(model: Node): string | undefined {
    return model.metadata && model.metadata.uid;
  }

  public static onWsDisconnect() {
    if (this.ws_disconnected) return;
    this.ws_disconnected = true;
    this.all().forEach((item: Node) => {
      item.ws_disconnected = true;
    });
    console.log('this.$eventBus.emit("::wsDisconnected", "Incident");');
  }

  // Attributes
  public get group(): string | undefined {
    return this.metadata.labels["node.deckhouse.io/group"];
  }

  public get state(): string {
    return this.status.conditions.find((c) => c.type == "Ready")?.status == "True" ? "Ready" : "NotReady";
  }

  public get zone(): string | undefined {
    return this.metadata.labels["topology.kubernetes.io/zone"];
  }

  public get internalIP(): string | undefined {
    return this.status.addresses.find((a) => a.type == "InternalIP")?.address;
  }

  public get externalIP(): string | undefined {
    return this.status.addresses.find((a) => a.type == "ExternalIP")?.address;
  }

  public get podCIDRs(): string[] | undefined {
    return this.spec.podCIDRs;
  }

  public get kubeletVersion(): string | undefined {
    return this.status.nodeInfo.kubeletVersion;
  }

  public get kubeproxyVersion(): string | undefined {
    return this.status.nodeInfo.kubeProxyVersion;
  }

  public get cri(): string | undefined {
    return this.status.nodeInfo.containerRuntimeVersion;
  }

  public get kernelVersion(): string | undefined {
    return this.status.nodeInfo.kernelVersion;
  }

  public get osImage(): string | undefined {
    return this.status.nodeInfo.osImage;
  }

  public get os(): string | undefined {
    return this.status.nodeInfo.operatingSystem;
  }

  public get arch(): string | undefined {
    return this.status.nodeInfo.architecture;
  }

  public get hostname(): string | undefined {
    return this.metadata.labels["kubernetes.io/hostname"];
  }

  public get machineID(): string | undefined {
    return this.status.nodeInfo.machineID;
  }

  public get systemUUID(): string | undefined {
    return this.status.nodeInfo.systemUUID;
  }

  public get bootID(): string | undefined {
    return this.status.nodeInfo.bootID;
  }

  public get unschedulable(): boolean {
    return this.spec.unschedulable == "true";
  }

  public get needDisruptionApproval(): boolean {
    return (
      this.metadata.annotations["update.node.deckhouse.io/disruption-required"] == "" &&
      !this.metadata.annotations["update.node.deckhouse.io/disruption-approved"]
      // TODO: check nodeGroup.metadata.annotations["disruption.mode"] == "Manual"
    );
  }

  public get errorMessage(): string | undefined {
    return this.status.conditions.find(({ type: t, status }) => t === "Error" && status === "True")?.message;
  }

  // Network functions

  public static async query(params: object = {}): Promise<Array<Node>> {
    return Promise.reject("Node:NotImplemented");
  }

  public static async get(params: object = {}): Promise<Node | null> {
    return Promise.reject("Node:NotImplemented");
  }

  private static async update(params: object = {}, object: object = {}): Promise<null> {
    return Promise.reject("Node:NotImplemented");
  }

  private static async drain(params: object = {}): Promise<null> {
    return Promise.reject("Node:NotImplemented");
  }

  public async save(): Promise<Node | null> {
    const attrs = (({ klassName, is_stale, ...o }) => o)(this);
    return Node.update({ name: this.metadata.name }, attrs);
  }

  public async drain(): Promise<Node | null> {
    return Node.drain({ name: this.metadata.name });
  }

  public async disruptionApprove(): Promise<Node | null> {
    this.metadata.annotations["update.node.deckhouse.io/disruption-approved"] = "";
    return this.save();
  }
}

// var resourceBaseUrl = `${window.location.protocol}//:hostname/:api_path/Nodes`;
const resourceBaseUrl = `k8s/nodes`;
Node.setRoutes(
  resourceBaseUrl,
  {},
  {
    query:  { method: "GET", storeResponse: true, queryCache: true, format: "array", withCredentials: false },
    get:    { method: "GET", url: resourceBaseUrl + "/:name", storeResponse: true, withCredentials: false },
    update: { method: "PUT", url: resourceBaseUrl + "/:name", storeResponse: false, withCredentials: false },
    drain:  { method: "PUT", url: resourceBaseUrl + "/:name/drain", storeResponse: false, withCredentials: false },
  },
  { dynamic_cache: false }
);
Node.initSubscription("GroupResourceChannel", { groupResource: "nodes" });

export default Node;

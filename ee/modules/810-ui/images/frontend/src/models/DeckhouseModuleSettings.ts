// @ts-ignore
import type { IUpdateWindow } from "@/types";
import NxnResourceWs from "@lib/nxn-common/models/NxnResourceWs";

interface DeckhouseModuleAttributes {
  apiVersion: string;
  kind: string;
  metadata: {
    uid: string;
    resourceVersion: string;
    [key: string]: string | object;
  };
  spec: {
    settings: DeckhouseSettings;
    [key: string]: string | object;
  };
  status: object;
}

export interface IDeckhouseModuleReleaseNotification {
  webhook?: string;
  minimalNotificationTime?: string;
  auth?: {
    basic?: { password: string; username: string };
    bearerToken?: string;
  };
}

export interface IDeckhouseModuleRelease {
  mode?: string;
  disruptionApprovalMode?: string;
  windows: IUpdateWindow[];
  notification?: IDeckhouseModuleReleaseNotification;
}

export class DeckhouseSettings {
  public bundle?: string;
  public logLevel?: string;
  public releaseChannel: string;
  public release: IDeckhouseModuleRelease;

  constructor({ bundle, logLevel, releaseChannel, release }: DeckhouseSettings) {
    this.bundle = bundle;
    this.logLevel = logLevel;
    this.releaseChannel = releaseChannel;
    this.release = release;
  }
}

class DeckhouseModuleSettings extends NxnResourceWs<DeckhouseModuleSettings> implements DeckhouseModuleAttributes {
  public static klassName: string = "DeckhouseModuleSettings";
  public is_stale: boolean = false;

  public apiVersion: string;
  public kind: string;
  public metadata: { [key: string]: string | object; uid: string; resourceVersion: string };
  public spec: { [key: string]: string | object; settings: DeckhouseSettings };
  public status: object;

  constructor(attrs: DeckhouseModuleAttributes) {
    super(attrs);
    this.apiVersion = attrs.apiVersion;
    this.kind = attrs.kind;
    this.metadata = attrs.metadata;
    this.spec = attrs.spec;
    this.status = attrs.status;

    // KOSTYL
    // this.spec.settings.release ||= {} as IDeckhouseModuleRelease;
  }

  public static toPrimaryKey(model: DeckhouseModuleSettings): string {
    return model?.metadata.uid;
  }

  public static toVersionKey(model: DeckhouseModuleSettings): string | undefined {
    return model.metadata?.resourceVersion;
  }

  public get settings(): DeckhouseSettings {
    return this.spec.settings;
  }

  public async save(): Promise<void> {
    const attrs = (({ is_stale, status, ...o }) => o)(this);
    return DeckhouseModuleSettings.update({}, attrs);
  }
}

const routes = {
  get: { method: "GET", storeResponse: true, withCredentials: false },
  update: { method: "PUT", storeResponse: false, withCredentials: false },
};

DeckhouseModuleSettings.setRoutes(`k8s/deckhouse.io/moduleconfigs/deckhouse`, {}, routes, { queryCache: true });
DeckhouseModuleSettings.initSubscription("GroupResourceChannel", { groupResource: "moduleconfigs.deckhouse.io" });

export default DeckhouseModuleSettings;

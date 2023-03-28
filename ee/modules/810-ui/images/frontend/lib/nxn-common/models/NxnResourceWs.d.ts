export default NxnResourceWs;
declare class NxnResourceWs<T extends NxnResourceWs> extends NxnResourceHttp<T> {
  static initSubscription(channelName: string, params?: {}): void;
  static subscribe(kwargs: any): any;
  static unsubscribe(): boolean;
  static getCable(forceCablePath: any): any;
  static createCable(cablePath: any): any;
}
import NxnResourceHttp from "./NxnResourceHttp.js";

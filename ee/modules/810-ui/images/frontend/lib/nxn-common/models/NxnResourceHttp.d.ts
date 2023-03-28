export default NxnResourceHttp;

declare class NxnResourceHttp<T extends NxnResourceHttp> extends NxnResourceDB<T> {
  static apiUrl(_: any, ...args: string[]): string;
  static setRoutes(defaultUrl: string, defaultUrlParams: object, apiActions: object, kwargs: any): void;
  static saveListServerRepresentation(listRepresentation: any[], saveSettings: any): T;
  static saveServerRepresentation(representation: any, saveSettings: any): T;
  static saveInconsequentialUpdate(representation: any, saveSettings: any): T;
  static addApiAction(name: any, actionDescr: any): void;
  constructor(attrs: any);
}
import NxnResourceDB from "./NxnResourceDB.js";

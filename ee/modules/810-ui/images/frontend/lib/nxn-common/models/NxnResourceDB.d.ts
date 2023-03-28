export default NxnResourceDB;
declare class NxnResourceDB<T extends NxnResourceDB> {
  static toPrimaryKey(model: T): string | undefined;
  static toVersionKey(model: T): string | undefined;
  static all(): T[];
  static active(): T[];
  static allPrimaryKeys(): string[];
  static delete_all(): void;
  static where(query: any): T[];
  static filter(checker: (model: T) => boolean): T[];
  static find(primaryKey: string): T;
  static find_by(query: any): T;
  static find_with(checker: (model: T) => boolean): T;
  static findOrCreateBy(attrs: any): T;
  static nxndbDestroy(primaryKey: string, kwargs: any): void;
  static nxndbSave(newVal: any, kwargs: any): false | T;
  static nxndbUpdate(newVal: any, kwargs: any): T;
  static nxndbCreate(newVal: any, kwargs: any): false | T;
  static addChannelCallback(messageType: any, callback: any): any;
  static removeChannelCallbacks(...args: any[]): boolean;
  static runChannelCallbacks(messageType: any, fnArgs: any, kwargs: any): void;
  static queryCacheOnDestroy(): void;
  static queryCacheOnCreate(): void;
  primaryKey(): string;
  versionKey(): string;
  nxndbSave(kwargs: any): false | T;
  nxndbCreate(kwargs: any): false | T;
  is_stale: boolean;
  ws_disconnected: boolean; // TODO: why it is here?
  nxndbUpdate(newVal: any, kwargs: any): T;
  putToStorage(): false | T;
  nxndbDestroy(kwargs?: any): () => void;
  removeFromStorage(): T;
  applyUnappliedUpdates(): void;
}
declare namespace NxnResourceDB {
  const storage: {};
  const klassName: any;
  namespace channelCallbacks {
    export const update: any[];
    export const create: any[];
    const _delete: any[];
    export { _delete as delete };
  }
}

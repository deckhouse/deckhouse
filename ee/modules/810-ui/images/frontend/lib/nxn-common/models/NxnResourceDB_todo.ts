// import 'nxn-common/shims.js' // TODO: somewhere else
import deepcopy from 'deepcopy/index.js'
import { reactive } from 'vue'

// requires toPrimaryKey, runChannelCallbacks

class nxndbUpdateKwargs {
  dontFlushQueryCache?: Boolean
  toUnappliedUpdatesIfNotStored?: Boolean
  noCallbacks?: Boolean
  dontCall?: Array<Function>
  messageData?: Object
}

abstract class NxnResourceDB {
  constructor(public attributes: any) {}

  // TODO: use some indexed storage
  public static klassName: string
  private static storage: Object
  public static unappliedUpdates: Object
  public static NxnDBMergeableAttrs: Array<string>
  public static dontRemoveFromStorageOnDelete: Boolean

  public static channelCallbacks = {
    update: [],
    create: [],
    delete: []
  }

  //@ts-ignore
  public is_stale: Boolean
  //@ts-ignore
  public metadata: Metadata

  //if (extraKwargs && Array.isArray(extraKwargs.mergeableAttrs) && extraKwargs.mergeableAttrs.length) {
  //  Resource.NxnDBMergeableAttrs = extraKwargs.mergeableAttrs
  //}

  // class methods
  public static toPrimaryKey<T extends typeof NxnResourceDB>(model: InstanceType<T>): string {
    return model.metadata.name + '-' + model.metadata.resourceVersion
  }

  public static allPrimaryKeys():Array<string> {
    return Object.keys(this.storage)
  }

  public static delete_all() {
    this.storage = {}
  }

  // SELECTORS return array
  public static all<T extends typeof NxnResourceDB>(): Array<InstanceType<T>> {
    var res = []
    var key
    for (key in this.storage) {
      if (this.storage.hasOwnProperty(key)) {
        //@ts-ignore
        res.push(this.storage[key])
      }
    }
    return res
  }

  public static where<T extends typeof NxnResourceDB>(query: any): Array<InstanceType<T>> {
    var res = []
    var hit:Boolean, model:T
    for (var primaryKey in this.storage) {
      //@ts-ignore
      model = this.storage[primaryKey]
      hit = true
      for (var attrName in query) {
        //@ts-ignore
        if (model[attrName] != query[attrName]) {
          hit = false
          break
        }
      }
      if (hit) { res.push(model) }
    }
    //@ts-ignore
    return res
  }

  public static filter<T extends typeof NxnResourceDB>(checker: Function): Array<InstanceType<T>> {
    var res = []
    var hit:Boolean, model:T
    for (var primaryKey in this.storage) {
      //@ts-ignore
      model = this.storage[primaryKey]
      if (checker(model)) { res.push(model) }
    }
    //@ts-ignore
    return res
  }

  // FINDERS return instance
  public static find<T extends typeof NxnResourceDB>(primaryKey: string): InstanceType<T> {
    //@ts-ignore
    return this.storage[primaryKey]
  }

  public static find_by<T extends typeof NxnResourceDB>(query: any): InstanceType<T> {
    var hit:Boolean, model:InstanceType<T>
    for (var primaryKey in this.storage) {
      //@ts-ignore
      model = this.storage[primaryKey]
      hit = true
      for (var attrName in query) {
        //@ts-ignore
        if (model[attrName] != query[attrName]) {
          hit = false
          break
        }
      }
      if (hit) { return model }
    }
    //@ts-ignore
    return
  }

  public static find_with<T extends typeof NxnResourceDB>(checker: Function): InstanceType<T> {
    var hit:Boolean, model:InstanceType<T>
    for (var primaryKey in this.storage) {
      //@ts-ignore
      model = this.storage[primaryKey]
      if (checker(model)) { return model }
    }
    //@ts-ignore
    return
  }

  public static findOrCreateBy<T extends typeof NxnResourceDB>(attrs: any): InstanceType<T> {
    return this.find_by(attrs) || this.nxndbSave(attrs)
  }

  // STORAGE OPERATIONS: static methods
  public static nxndbDestroy(primaryKey: string, kwargs: nxndbUpdateKwargs = {}) {
    var model = this.find(primaryKey)
    if (model) {
      model.nxndbDestroy(kwargs)
    } else {
      this.queryCacheOnDestroy()
    }
    return
  }

  public static nxndbSave<T extends typeof NxnResourceDB>(newVal: any, kwargs: nxndbUpdateKwargs = {}): InstanceType<T> {
    var res = this.nxndbUpdate(newVal, kwargs)

    if (res instanceof String) {
      if (res === null) {
        // 'not_found'
        //@ts-ignore
        return this.nxndbCreate(newVal, kwargs)
      } else {
        //@ts-ignore
        return null
      }
    } else {
      return res as InstanceType<T>
    }
  }

  public static nxndbUpdate<T extends typeof NxnResourceDB>(newVal: any, kwargs: nxndbUpdateKwargs = {}): InstanceType<T> {
    var primaryKey = this.toPrimaryKey(newVal)
    var existingModel = this.find(primaryKey)

    if (primaryKey && !!existingModel) {
      return existingModel.nxndbUpdate(newVal, kwargs)

    } else if (primaryKey && !existingModel && kwargs && kwargs.toUnappliedUpdatesIfNotStored) {
      //@ts-ignore
      if (!this.unappliedUpdates[primaryKey]) {
        //@ts-ignore
        this.unappliedUpdates[primaryKey] = []
      }
      //@ts-ignore
      this.unappliedUpdates[primaryKey].push(newVal)
      //@ts-ignore
      return undefined
    }
    //@ts-ignore
    return null
  }

  public static nxndbCreate<T extends typeof NxnResourceDB>(this: { new(p:any): T }, newVal: any, kwargs: nxndbUpdateKwargs = {}): InstanceType<T> {
    var model = (newVal instanceof this) ? newVal : new this(newVal)
    //@ts-ignore
    return model.nxndbCreate(kwargs)
  }

  // STORAGE OPERATIONS: instance methods
  public nxndbSave<T extends typeof NxnResourceDB>(kwargs: nxndbUpdateKwargs = {}): InstanceType<T> {
    return (this.constructor as T).nxndbSave(this, kwargs)
  }

  public nxndbCreate<T extends typeof NxnResourceDB>(kwargs: nxndbUpdateKwargs = {}): InstanceType<T> {
    if (kwargs && !kwargs.dontFlushQueryCache) {
      (this.constructor as T).queryCacheOnCreate()
    }
    if (!this.putToStorage()) {
      //@ts-ignore
      return null
    }
    //@ts-ignore
    this.applyUnappliedUpdates()
    //@ts-ignore
    if (!this.is_stale) this.is_stale = false
    (this.constructor as T).runChannelCallbacks('create', [this], kwargs)
    //@ts-ignore
    return this
  }

  public nxndbUpdate<T extends typeof NxnResourceDB>(newVal: any, kwargs: nxndbUpdateKwargs = {}): InstanceType<T> {
    if (!this.metadata.resourceVersion || !newVal.metadata.resourceVersion || (this.metadata.resourceVersion <= newVal.metadata.resourceVersion)) {
      var preChangeCopy = deepcopy(this) // WARNING doesn't copy functions!
      //@ts-ignore
      var klassNxnDBMergeableAttrs = klassNxnDBMergeableAttrs()
      if (!klassNxnDBMergeableAttrs) {
        Object.assign(this, newVal)
      } else {
        //@ts-ignore
        Object.assign(this, Object.copyExcept(newVal, klassNxnDBMergeableAttrs))
        klassNxnDBMergeableAttrs.forEach((a:string) => {
          if (newVal[a] === undefined) return
          //@ts-ignore
          if (this[a]) {
            //@ts-ignore
            Object.assign(this[a], newVal[a])
          } else {
            //@ts-ignore
            this[a] = newVal[a]
          }
        })
      }
      (this.constructor as T).runChannelCallbacks('update', [this, preChangeCopy], kwargs)
    }
    //@ts-ignore
    return this
  }

  public putToStorage<T extends typeof NxnResourceDB>(): InstanceType<T> {
    var primaryKey = (this.constructor as T).toPrimaryKey(this)
    if (primaryKey) {
      //@ts-ignore
      (this.constructor as T).storage[primaryKey] = this
      //@ts-ignore
      return this
    } else {
      console.warn("NxnDB tried to store Resource without primaryKey")
      console.warn("   this = " + JSON.stringify(this))
      //@ts-ignore
      return null
    }
  }

  public nxndbDestroy<T extends typeof NxnResourceDB>(kwargs: nxndbUpdateKwargs = {}): InstanceType<T> | Function {
    (this.constructor as T).queryCacheOnDestroy()
    if (!(this.constructor as T).dontRemoveFromStorageOnDelete) {
      //@ts-ignore
      this.removeFromStorage()
    }
    if (!!kwargs && !!kwargs.messageData && (Object.keys(kwargs.messageData).length > 1)) {
      // means that it includes update data that can be needed by onDelete callbacks
      this.nxndbUpdate(kwargs.messageData, { noCallbacks: true })
    }

    (this.constructor as T).runChannelCallbacks('delete', [this], kwargs)

    var rollbacker = function() {
    //@ts-ignore
    if (!!this.nxndbSave) {
      //@ts-ignore
      this.nxndbSave(kwargs)
      } else {
        console.warn("Can't roll back with recreating deleted object: it does't have nxndbSave function") // WHY it doesn't?!
      }
    }
    return rollbacker
  }

  public removeFromStorage<T extends typeof NxnResourceDB>(this: T): InstanceType<T> {
    //@ts-ignore
    delete (this.constructor as T).storage[(this.constructor as T).toPrimaryKey(this)]
    //@ts-ignore
    return this
  }

  // WARNING: this method doesn't call channelCallbacks!
  public applyUnappliedUpdates<T extends typeof NxnResourceDB>(this: T): void {
    if (this.unappliedUpdates) {
      var primaryKey, updatesQueue
      //@ts-ignore
      primaryKey = (this.constructor as T).toPrimaryKey(this)
      //@ts-ignore
      updatesQueue = this.unappliedUpdates[primaryKey]

      if (!!updatesQueue) {
        //@ts-ignore
        updatesQueue.sort(function(a,b){ return a.metadata.resourceVersion - b.metadata.resourceVersion })

        var j, len
        for (j = 0, len = updatesQueue.length; j < len; j++) {
          this.nxndbUpdate(updatesQueue[j], { noCallbacks: true })
        }

        //@ts-ignore
        delete this.unappliedUpdates[primaryKey]
      }
    }

    return
  }

  public static addChannelCallback(messageType: string, callback: Function): Function {
    if (callback == undefined) console.error('Tried to add undefined callback to ' + this.klassName)
    //@ts-ignore
    if (!this.channelCallbacks[messageType]) {
      //@ts-ignore
      this.channelCallbacks[messageType] = []
    }
    //@ts-ignore
    this.channelCallbacks[messageType].push(callback)
    return callback
  }

  public static removeChannelCallbacks(): void {
    Array.from(arguments).forEach((cbReference) => { if (cbReference){ cbReference.isDeprecatedCB = true } })
    Object.keys(this.channelCallbacks).forEach((messageType) => {
    //@ts-ignore
    if (!!this.channelCallbacks && !!this.channelCallbacks[messageType]) {
        //@ts-ignore
        this.channelCallbacks[messageType] = this.channelCallbacks[messageType].filter((cb) => { return !cb.isDeprecatedCB })
      }
    })
  }

  // private

  private static runChannelCallbacks(messageType: string, fnArgs:any, kwargs: nxndbUpdateKwargs = {}): void {
    if (!this.channelCallbacks || !!kwargs && kwargs.noCallbacks) {
      return
    }
    //@ts-ignore
    if (!this.channelCallbacks[messageType]) {
      console.warn("Unexpected message type:" + String(messageType))
      return
    }

    var exceptions = kwargs && kwargs.dontCall || []
    //@ts-ignore
    this.channelCallbacks[messageType].filter((fn) => {
      return exceptions.indexOf(fn) < 0
    }).forEach((fn:any) => {
      fn.apply(this, fnArgs.concat(kwargs))
    })
  }


  private static queryCacheOnDestroy(): void {
  }

  private static queryCacheOnCreate(): void {
  }

  // .TS TRASH DUPLICATED IN EVERY CLASS because github.com/Microsoft/TypeScript/issues/7673, github.com/Microsoft/TypeScript/issues/
  // `(typeof this).staticMethod`  - does not work
  private klassNxnDBMergeableAttrs(): Array<string> {
    return NxnResourceDB.NxnDBMergeableAttrs
  }
}

export default NxnResourceDB

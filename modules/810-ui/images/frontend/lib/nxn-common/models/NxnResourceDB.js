// import 'nxn-common/shims.js' // TODO: somewhere else
// no npm install => no deepcopy; import * as deepcopy from 'deepcopy'; //import deepcopy from 'deepcopy/index.js'

class NxnResourceDB {
  // class methods
  static toPrimaryKey(model) {
    return model.metadata && (model.metadata.name + '-' + model.metadata.resourceVersion);
  }

  static all() {
    var res = [];
    var key;
    for (key in this.storage) {
      if (this.storage.hasOwnProperty(key)) {
        res.push(this.storage[key]);
      }
    }
    return res;
  }

  static active() {
    return this.where({ archived_at: null }); // won't work on all resources, but still useful method
  }

  static allPrimaryKeys() {
    return Object.keys(this.storage);
  }

  static delete_all() {
    this.storage = {};
  }

  static where(query) {
    var res = [];
    var hit, model;
    for (var primaryKey in this.storage) {
      model = this.storage[primaryKey];
      hit = true;
      for (var attrName in query) {
        if (model[attrName] != query[attrName]) {
          hit = false;
          break;
        }
      }
      if (hit) { res.push(model); }
    }
    return res;
  }

  static filter(checker) {
    var res = [];
    var hit, model;
    for (var primaryKey in this.storage) {
      model = this.storage[primaryKey];
      if (checker(model)) { res.push(model); }
    }
    return res;
  }

  static find(primaryKey) {
    return this.storage[primaryKey];
  }

  static find_by(query) {
    var hit, model;
    for (var primaryKey in this.storage) {
      model = this.storage[primaryKey];
      hit = true;
      for (var attrName in query) {
        if (model[attrName] != query[attrName]) {
          hit = false;
          break;
        }
      }
      if (hit) { return model; }
    }
    return;
  }

  static find_with(checker) {
    var hit, model;
    for (var primaryKey in this.storage) {
      model = this.storage[primaryKey];
      if (checker(model)) { return model; }
    }
    return;
  }

  static findOrCreateBy(attrs) {
    return db.find_by(attrs) || this.nxndbSave(attrs);
  }

  static nxndbDestroy(primaryKey, kwargs) {
    var model = this.find(primaryKey);
    if (model) {
      model.nxndbDestroy(kwargs);
    } else {
      this.queryCacheOnDestroy();
    }
    return;
  }

  static nxndbSave(newVal, kwargs) {
    var res = this.nxndbUpdate(newVal, kwargs);

    if (res === 'unprocerssed') {
      return this.nxndbCreate(newVal, kwargs);
    } else {
      return res;
    }
  }

  static nxndbUpdate(newVal, kwargs) {
    var primaryKey = this.toPrimaryKey(newVal);
    var existingModel = this.find(primaryKey);

    if (primaryKey && !!existingModel) {
      return existingModel.nxndbUpdate(newVal, kwargs);

    } else if (primaryKey && !existingModel && kwargs && kwargs.toUnappliedUpdatesIfNotStored) {
      if (!this.unappliedUpdates[primaryKey]) {
        this.unappliedUpdates[primaryKey] = [];
      }
      this.unappliedUpdates[primaryKey].push(newVal);
      return 'unappliedUpdate';
    }
    return 'unprocerssed';
  }

  static nxndbCreate(newVal, kwargs) {
    var model = (newVal instanceof this) ? newVal : new this(newVal);
    return model.nxndbCreate(kwargs);
  }

  // instance methods
  // TODO: stop using instance method?
  primaryKey() {
    return this.constructor.toPrimaryKey(this);
  }

  nxndbSave(kwargs) {
    return this.constructor.nxndbSave(this, kwargs);
  }

  nxndbCreate(kwargs) {
    if (kwargs && !kwargs.dontFlushQueryCache) {
      this.constructor.queryCacheOnCreate();
    }
    if (!this.putToStorage()) {
      return false;
    }
    this.applyUnappliedUpdates();
    if (typeof (this.is_stale) === 'undefined') this.is_stale = false;
    if (this.constructor.ws_disconnected) this.ws_disconnected = true;
    this.constructor.runChannelCallbacks('create', [this], kwargs);
    return this;
  }

  nxndbUpdate(newVal, kwargs) {
    if (!this.metadata.resourceVersion || !newVal.metadata.resourceVersion || (this.metadata.resourceVersion <= newVal.metadata.resourceVersion)) {
      var preChangeCopy = Object.assign({}, this) // deepcopy(this); // WARNING doesn't copy functions anymore!
      if (!this.constructor.NxnDBMergeableAttrs) {
        Object.assign(this, newVal);
      } else {
        Object.assign(this, Object.copyExcept(newVal, this.constructor.NxnDBMergeableAttrs));
        var self = this;
        this.constructor.NxnDBMergeableAttrs.forEach(function(a){
          if (newVal[a] === undefined) return;
          if (self[a]) {
            Object.assign(self[a], newVal[a]);
          } else {
            self[a] = newVal[a];
          }
        });
      }
      this.constructor.runChannelCallbacks('update', [this, preChangeCopy], kwargs);
    }
    return this;
  }

  putToStorage() {
    var primaryKey = this.constructor.toPrimaryKey(this);
    if (primaryKey) {
      this.constructor.storage[primaryKey] = this;
      return this;
    } else {
      console.warn("NxnDB tried to store Resource without primaryKey");
      console.warn("   this = " + JSON.stringify(this));
      return false;
    }
  }

  nxndbDestroy(kwargs) {
    this.constructor.queryCacheOnDestroy();
    if (!this.constructor.dontRemoveFromStorageOnDelete) {
      this.removeFromStorage();
    }
    if (!!kwargs && !!kwargs.messageData && (Object.keys(kwargs.messageData).length > 1)) {
      // means that it includes update data that can be needed by onDelete callbacks
      this.nxndbUpdate(kwargs.messageData, { noCallbacks: true });
    }

    this.constructor.runChannelCallbacks('delete', [this], kwargs);

    var rollbacker = function() {
      if (!!self.nxndbSave) {
        self.nxndbSave(kwargs);
      } else {
        console.warn("Can't roll back with recreating deleted object: it does't have nxndbSave function"); // WHY it doesn't?!
      }
    }
    return rollbacker;
  }

  removeFromStorage() {
    delete this.constructor.storage[this.constructor.toPrimaryKey(this)];
    return this;
  }

  // WARNING: this method doesn't call channelCallbacks!
  applyUnappliedUpdates() {
    if (this.unappliedUpdates) {
      var primaryKey, updatesQueue;
      primaryKey = this.constructor.toPrimaryKey(this);
      updatesQueue = this.unappliedUpdates[primaryKey];

      if (!!updatesQueue) {
        updatesQueue.sort(function(a,b){ return a.updated_at_f - b.updated_at_f; });

        var j, len;
        for (j = 0, len = updatesQueue.length; j < len; j++) {
          this.nxndbUpdate(updatesQueue[j], { noCallbacks: true });
        }

        delete this.unappliedUpdates[primaryKey];
      }
    }

    return;
  }

  // private

  static runChannelCallbacks(messageType, fnArgs, kwargs) {
    if (!this.channelCallbacks || !!kwargs && kwargs.noCallbacks) {
      return;
    }
    if (!this.channelCallbacks[messageType]) {
      console.warn("Unexpected message type:" + String(messageType));
      return;
    }

    var exceptions = kwargs && kwargs.dontCall || [];
    var self = this;
    this.channelCallbacks[messageType].filter(function(fn) {
      return exceptions.indexOf(fn) < 0;
    }).forEach(function(fn) {
      fn.apply(self, fnArgs.concat(kwargs));
    });
  }

  static queryCacheOnDestroy() {
  }

  static queryCacheOnCreate() {
  }
}

NxnResourceDB.klassName = null;
NxnResourceDB.channelCallbacks = {
  update: [],
  create: [],
  delete: []
}
// optional: NxnDBMergeableAttrs

export default NxnResourceDB;

// Even more primitive query cache - for models that only use local filtering
export default {
  noQueryFilters: true,

  flushQueryCache() {
    this.queryCache = [];
  },

  queryCacheOnCreate() {},

  queryCacheOnDestroy() {},

  cachedResultFor(actionDescr, actionName, params) {
    var cached = this.queryCache.find((i) => { return actionName == i.actionName; });
    if (cached) {
      return cached.result.then((resp) => { return this.all(); });
    } else {
      return undefined;
    }
  },

  pushToQueryCache(actionName, newPromise, params) {
    this.queryCache.push({ actionName: actionName, result: newPromise });
  },

  channelChangeParams(channel, newParams) {
    console.error("Can't call channelChangeParams for model with primitive cache that only uses local filtering");
  }
}

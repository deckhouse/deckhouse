import deepEqual from "fast-deep-equal";

// Primitive query cache
export default {
  flushQueryCache() {
    this.queryCache = [];
  },

  queryCacheOnCreate() {
    this.flushQueryCache();
  },

  queryCacheOnDestroy() {
    this.flushQueryCache();
  },

  // TODO: hadle case of: console.warn(`${this.klassName}.queryCache got invalidated before request resolution`);

  // WARNING: 'GETs only' restriction removed because some requests don't not use POST
  cachedResultFor(actionDescr, actionName, params) {
    var cached;
    if (cached = this.findInQueryCache(actionName, params)) {
      return cached;
    } else {
      return undefined;
    }
  },

  findInQueryCache(actionName, params) {
    var i, j, len;
    for (j = 0, len = this.queryCache.length; j < len; j++) {
      var cachedItem = this.queryCache[j];
      if ((actionName == cachedItem.actionName) && deepEqual(params || {}, cachedItem.params)) {
        return cachedItem.result;
      }
    }
    return;
  },

  pushToQueryCache(actionName, newPromise, params) {
    this.queryCache.push({ actionName: actionName, result: newPromise, params: params });
  },

  channelChangeParams(channel, newParams) {
    return channel.perform('change_params', newParams);
  }
}

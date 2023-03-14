// TODO: cleaner implementation and use of mixins in nxn?
// this patch adds (primitive) query cache to ngResource
export default function NxnQueryCache(Resource) {
  Resource.queryCache = []; // TODO: just give custom cache ot resource or use $httpDefaultCache=$cacheFactory.get('$http')

  Resource.flushQueryCache = function() {
    this.queryCache = [];
  };

  Resource.queryCacheOnCreate = function() {
    this.flushQueryCache();
  };

  Resource.queryCacheOnDestroy = function() {
    this.flushQueryCache();
  };

  // TODO: hadle case of: console.warn(`${Resource.klassName}.queryCache got invalidated before request resolution`);

  // WARNING: 'GETs only' restriction removed because some requests don't not use POST
  Resource.cachedResultFor = function(action, name, params) {
    var cached;
    if (cached = this.findInQueryCache(name, params)) {
      return cached;
    } else {
      return undefined;
    }
  };

  Resource.findInQueryCache = function(actionName, params) {
    var i, j, len;
    for (j = 0, len = this.queryCache.length; j < len; j++) {
      var cachedItem = this.queryCache[j];
      if ((actionName == cachedItem.actionName) && equal(params, cachedItem.params)) {
        return cachedItem.result;
      }
    }
    return;
  };

  Resource.pushToQueryCache = function(name, params, newPromise) {
    this.queryCache.push({ actionName: name, params: params, result: newPromise });
  };

  Resource.channelChangeParams = function(channel, newParams) {
    return channel.perform('change_params', newParams);
  };

  var onWsDisconnectCall = Resource.onWsDisconnect;
  Resource.onWsDisconnect = function(scope) {
    this.flushQueryCache();
    if (onWsDisconnectCall) onWsDisconnectCall.call(Resource, scope);
  };
}

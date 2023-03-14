import deepcopy from 'deepcopy/index.js';

// Some values in oldParams/newParams are arrays: recombine them into multiple filters with only one value for each key
function recombineParamsTo1k1v(params) {
  return Object.keys(params).reduce((acc, key) => {
    var values = params[key];
    if (!Array.isArray(values)) values = [values];

    if (!acc.length) return values.map((value) => { var filter = {}; filter[key] = value; return filter; });

    // permute accumulated filters with this array of values
    var newFilters = [];
    acc.forEach((filter) => {
      for (var i = 1; i < values.length; i++) {
        var newFilter = Object.assign({}, filter);
        newFilter[key] = values[i];
        newFilters.push(newFilter);
      }
      filter[key] = values[0];
    });

    return acc.concat(newFilters);
  }, []);
}

function filter1k1vFitsParams(target, params) {
  return recombineParamsTo1k1v(params).some((filter) => { return equal(target, filter); });
}

// New cache.
// Maintains list of items for every made request using subscription callbacks.
// List is given out as 'cached' request result.
// This removes necessity to invalidate cache on every create/delete.
// 'Cache' is invalidated only in parts and by unsubscription or disconnect or subscription params change.
export default function NxnQueryCacheDynamic(Resource, cacheKwargs) {
  // Removal from storage on delete can't be used because of conflicting subscriptions,
  // this Resource has to use garbageCollectStorageUsingDynamicCache().
  Resource.dontRemoveFromStorageOnDelete = true;
  Resource.cacheKeyParamsWhitelist = cacheKwargs.cacheKeyParamsWhitelist;
  Resource.queryCache = {};

  Resource.flushQueryCache = function(channel) {
    if (!channel) return console.error('NxnQueryCacheDynamic.flushQueryCache called without channel. NxnQueryCacheDynamic can only be flushed in parts.');
    delete this.queryCache[channel.cacheKey];
  };

  Resource.queryCacheOnCreate = function() {
  };

  Resource.queryCacheOnDestroy = function() {
  };

  // WARNING: 'GETs only' restriction removed because some requests don't not use POST
  Resource.cachedResultFor = function(actionDescr, name, params, originalArguments) {
    var cached;
    if (cached = this.queryCache[this.queryParamsToCacheKey(params)]) {
      return cached.promise.then((resp) =>{
        var resolvedCached = Resource.queryCache[Resource.queryParamsToCacheKey(params)]

        if (resolvedCached) {
          return [...resolvedCached.items]

        } else {
          // TODO: do commented below if we verify that, by now, new subscription (for these params) was created
          // console.warn(`...`);
          // Resource[name].call(originalArguments);
          // return Resource.cachedResultFor(actionDescr, name, params, originalArguments);
          console.error(`${Resource.klassName}.queryCache got invalidated before request resolution`);
          return Promise.reject(`${Resource.klassName}.queryCache got invalidated before request resolution`);
        }
      });
    } else {
      return undefined;
    }
  };

  Resource.filterRelevantQueryParams = function(params) {
    var src = deepcopy(params || {});
    return Object.assign(
      {},
      Resource.cacheKeyParamsWhitelist.reduce((acc, k) => {
        if (src[k] !== undefined) acc[k] = src[k];
        return acc;
      }, {})
    );
  };

  // WARNING: depends JSON.stringify for param value
  Resource.queryParamsToCacheKey = function(params) {
    return this.cacheKeyParamsWhitelist
               .filter((k) => { return params[k] !== undefined; })
               .sort()
               .map((k) => { return `${k}:${JSON.stringify(params[k])}`; })
               .join(', ');
  };

  Resource.pushToQueryCache = function(name, params, newPromise) {
    var cacheKey = this.queryParamsToCacheKey(params);
    this.queryCache[cacheKey] = {
      promise: newPromise.then((resp) => {
        var cached = Resource.queryCache[cacheKey];
        if (cached) cached.items = cached.items.concat(resp);
        return resp;
      }),
      queryParams: this.filterRelevantQueryParams(params),
      items: []
    };
  };

  var onWsDisconnectCall = Resource.onWsDisconnect;
  Resource.onWsDisconnect = function(channel) {
    delete this.queryCache[channel.cacheKey];
    if (onWsDisconnectCall) onWsDisconnectCall.call(Resource);
  };

  var subscribeCall = Resource.subscribe;
  Resource.subscribe = function(kwargs) {
    var channel = subscribeCall.call(Resource, kwargs);
    channel.queryParams = this.filterRelevantQueryParams((kwargs || {}).params);
    channel.cacheKey = this.queryParamsToCacheKey(channel.queryParams);
    return channel;
  };

  Resource.unsubscribe = function(channel) {
    this.flushQueryCache(channel);
    return channel.unsubscribe();
  };

  Resource.channelChangeParams = function(channel, newParams) {
    // Some values in oldParams/newParams are arrays: recombine them into multiple filters with only one value for each attr
    var newQueryParams = this.filterRelevantQueryParams(newParams)
    var oldFilters1k1v = recombineParamsTo1k1v(channel.queryParams);
    var newFilters1k1v = recombineParamsTo1k1v(newQueryParams);
    var removedFilters1k1v = oldFilters1k1v.filter((oldFilter) => { return newFilters1k1v.every((newFilter) => { return !equal(newFilter, oldFilter); }); });

    // 1) {team: 1, status: ['a']}      -> {team: 1, status: ['b']}   INVALIDATE: {team: 1, status: ['a']}
    // 2) {team: 1, status: ['a', 'b']} -> {team: 1, status: ['b']}   INVALIDATE: {team: 1, status: ['a']}
    // 3) {team: 1, status: ['a', 'b']} -> {team: 1, status: ['c']}   INVALIDATE: {team: 1, status: ['a']}, {team: 1, status: ['b']}
    removedFilters1k1v.forEach((removedFilter) => {
      Object.keys(Resource.queryCache).filter((cacheKey) => {
        return filter1k1vFitsParams(removedFilter, Resource.queryCache[cacheKey].queryParams);
      }).forEach((cacheKey) => {
        delete Resource.queryCache[cacheKey];
      });
    });

    channel.cacheKey = this.queryParamsToCacheKey(channel.queryParams);
    channel.queryParams = newQueryParams;
    return channel.perform('change_params', newParams);
  };

  // WARNING: NxnQueryCacheDynamic requires channel in extraKwargs to perform.
  // WARNING: NxnQueryCacheDynamic is incompatible with app that excludes some updates (the ones made by this client) from streams.
  Resource.addChannelCallback('pure_channel_create', function(item, extraKwargs) {
    if (!extraKwargs || !extraKwargs.channel) return console.warn('NxnQueryCacheDynamic requires channel in extraKwargs to perform.');
    var cached = Resource.queryCache[extraKwargs.channel.cacheKey];
    if (!cached) return;
    if (!cached.items.some(function(c){ return Resource.toPrimaryKey(c) == Resource.toPrimaryKey(item); })) {
      cached.items.push(item);
    }
  });

  Resource.addChannelCallback('pure_channel_delete', function(deletedItemUuid, extraKwargs) {
    if (!extraKwargs || !extraKwargs.channel) return console.warn('NxnQueryCacheDynamic requires channel in extraKwargs to perform.');
    var cached = Resource.queryCache[extraKwargs.channel.cacheKey];
    if (!cached) return;

    var ref = cached.items;
    var i, j, len;
    for (i = j = 0, len = ref.length; j < len; i = ++j) {
      if (this.toPrimaryKey(ref[i]) === deletedItemUuid) {
        var item = cached.items.splice(i, 1)[0];
        break;
      }
    }
  });

  Resource.garbageCollectStorageUsingDynamicCache = function() {
    this.allGcRelevantItems().forEach((item) => {
      if (!Resource.isIdPresentInAnyCache(Resource.toPrimaryKey(item))) item.deleteByGc();
    });
  };

  Resource.isIdPresentInAnyCache = function(id) {
    return Object.keys(this.queryCache).some((cacheKey) => {
      return Resource.queryCache[cacheKey].items.some((item) => { return Resource.toPrimaryKey(item) === id; });
    });
  };

  if (!Resource.allGcRelevantItems) console.error("NxnQueryCacheDynamic requires 'allGcRelevantItems' class method");
}

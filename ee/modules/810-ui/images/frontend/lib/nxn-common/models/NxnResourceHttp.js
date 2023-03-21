import axios from "axios";
// import NxnQueryCacheDynamic from '../services/NxnQueryCacheDynamic.js';
import NxnQueryCache from './NxnQueryCache.js';
import NxnQueryCacheNoFilters from './NxnQueryCacheNoFilters.js';
import NxnResourceDB from './NxnResourceDB.js';

function wrapInKey(key, subject) {
  var res = {};
  res[key] = subject;
  return res;
}

function addWrappedInKey(data, key, subject) {
  return Object.assign({}, data, wrapInKey(key, subject));
}

class NxnResourceHttp extends NxnResourceDB {
  constructor(attrs) {
    super()
    Object.assign(this, attrs);
  }

  static apiUrl(_) {
    let paths = arguments;
    let baseUrl = this.baseUrl || `${window.location.protocol}//${window.location.hostname}`;
    return [baseUrl, ...paths].filter(function (e) { return e }).join('/');
  }

  static setRoutes(defaultUrl, defaultUrlParams, apiActions, kwargs) {
    if (kwargs && kwargs.queryCache) {
      // TODO: somehow make NxnQueryCache do it itself
      Object.assign(NxnResourceHttp, kwargs.noQueryFilters ? NxnQueryCacheNoFilters : NxnQueryCache);
      this.queryCache = [];
      var onWsDisconnectCall = this.onWsDisconnect;
      this.onWsDisconnect = function(scope) {
        this.flushQueryCache();
        if (onWsDisconnectCall) onWsDisconnectCall.call(this, scope);
      }
    };

    this.defaultUrl = defaultUrl;
    this.defaultUrlParams = defaultUrlParams || {};
    for (let key in apiActions) {
      this.addApiAction(key, apiActions[key]);
    }
  }

  static saveListServerRepresentation(listRepresentation, saveSettings) {
    return listRepresentation.map((representation) => {
      return this.saveServerRepresentation(representation, saveSettings);
    });
  }

  static saveServerRepresentation(representation, saveSettings) {
    return this.nxndbSave(representation, saveSettings);
  }

  static saveInconsequentialUpdate(representation, saveSettings) {
    return this.nxndbUpdate(representation, Object.assign({}, saveSettings, { noCallbacks: true }));
  }

  static addApiAction(name, actionDescr) {
    var klass = this;

    this[name] = function() {
      var params = arguments[0];
      var hasBody = /^(POST|PUT|PATCH)$/i.test(actionDescr.method);
      var data;
      if (hasBody) {
        data = arguments[1];
      }

      var cachedResult = actionDescr.queryCache ? klass.cachedResultFor(actionDescr, name, params, arguments) : undefined;
      if (cachedResult) {
        return Promise.resolve(cachedResult);
      }

      var config = { method: actionDescr.method.toLowerCase(), params: {}, withCredentials: actionDescr.withCredentials };
      if (hasBody) config.data = data;
      config.url = klass.apiUrl(actionDescr.url || klass.defaultUrl);

      for (let paramName in params) {
        if (params.hasOwnProperty(paramName)) {
          let regexp = new RegExp(':' + paramName + '(\/|:|$|#)', 'g');
          if (config.url.match(regexp)) {
            if (!!params[paramName]) {
              config.url = config.url.replace(regexp, function(match, p1) { return encodeURI(params[paramName]) + p1; });
            } else {
              // Eat '/' preceding empty param
              config.url = config.url.replace(new RegExp('\/?' + regexp.source), function(match, p1) { return p1; });
            }
          } else {
            // Encode "unsafe" (but not "reserved"!) characters that (except for blank space) axios encoder ignores.
            // blank space axios will encode as `+` and it should be good enough for our servers.
            let paramVal = params[paramName];
            if (typeof paramVal == 'string') {
              paramVal = paramVal.replace(/[<>#%{}|\^~\[\]`]/gi, function(c) {
                return encodeURIComponent(c);
              });
            }
            config.params[paramName] = paramVal;
          }
        }
      }

      var newPromise = axios.request(config).then((resp) => {
        var saveSettings = { noCallbacks: true, dontFlushQueryCache: true };
        var newResponse, array;

        if (actionDescr.storeResponse) {
          if (!actionDescr.format) {
            return klass.nxndbSave(resp.data, saveSettings);

          } else if (actionDescr.format == 'array') {
            return klass.saveListServerRepresentation(resp.data, saveSettings);

          } else if (actionDescr.format.arrayIn) {
            array = klass.saveListServerRepresentation(resp.data[actionDescr.format.arrayIn], saveSettings);
            return actionDescr.format.returnArray ? array : addWrappedInKey(resp.data, actionDescr.format.arrayIn, array);
          }

        } else {
          if (!!actionDescr.format && actionDescr.format.arrayIn) {
            array = resp.data[actionDescr.format.arrayIn];
            return actionDescr.format.returnArray ? array : addWrappedInKey(resp.data, actionDescr.format.arrayIn, array);
          } else {
            return resp.data;
          }
        }
      });

      if (actionDescr.queryCache) klass.pushToQueryCache(name, newPromise, Object.assign({}, params, data));

      return newPromise;
    };
  } // addApiAction
}

export default NxnResourceHttp;

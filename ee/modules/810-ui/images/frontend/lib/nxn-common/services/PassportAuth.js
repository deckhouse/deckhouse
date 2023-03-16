import XCSRFTokens from './XCSRFTokens.js';
import CurrentEditSession from './CurrentEditSession.js';
import GlobalNxnFlash from './GlobalNxnFlash.js';

var PassportAuth = {
  authPromise: undefined,
  params: undefined,

  baseUrl: function() {
    return window.location.protocol + '//' + this.params.api_url;
  },

  authUrl: function() {
    return this.baseUrl() + this.params.auth_path;
  },

  authorize: function(debugTag) {
    var authorizer = this;
    if (authorizer.authPromise === undefined) {
      authorizer.authPromise = axios.request({
        method: 'post',
        url: this.authUrl(),
        params: {
          passport: authorizer.passport,
          by: CurrentEditSession.id
        },
        withCredentials: true
      }).then(
        (resp) => {
          console.log('PassportAuth-Success');
          if (resp.data && !!resp.data.xcsrf_token) {
            XCSRFTokens[authorizer.baseUrl()] = resp.data.xcsrf_token;
          }
          if ((typeof resp.data == 'undefined') || (typeof resp.data.current_user == 'undefined')) {
            GlobalNxnFlash.show('error', authorizer.tag + ' failed to return current user', 0, authorizer.tag+'_auth');
            return Promise.reject(resp);
          } else {
            authorizer.authSucceeded = true;
            return authorizer.onAuth ? authorizer.onAuth.call(authorizer, resp) : resp;
          }
        },
        (resp) => {
          GlobalNxnFlash.show('error', 'Failed to authorize in ' + authorizer.tag, 0, 'passport_auth');
          return Promise.reject('PassportAuth-failed-to-authorize' + '-' + debugTag);
        }
      ).catch((error) => {
        authorizer.authSucceeded = false;
        return Promise.reject(error);
      });
    }
    return authorizer.authPromise;
  },

  // WARNING: do not put in chain too high - components never expect `undefined` as response to successfull action
  failSkipper: (resp) => {
    if ((typeof resp === 'string') && !!resp.match(/^PassportAuth-failed-to-authorize/)) {
      return Promise.resolve(null);
    } else {
      return Promise.reject(resp);
    }
  },

  wrap: function(action, forcedParams, kwargs, debugTag) {
    var authorizer = this;
    return function() {
      var subject = this; // 'model instance'
      var argumentsArr = Array.from(arguments);
      var actionArguments = [Object.assign({hostname: authorizer.params.api_url}, (forcedParams || {}), (argumentsArr[0] || {}))].concat(argumentsArr.slice(1, argumentsArr.length));

      var res = authorizer.authorize(debugTag).then(() => {
        return action.apply(subject, actionArguments);
      });
      if (!!kwargs && kwargs.dontPropagateAuthFail) {
        res = res.catch(authorizer.failSkipper);
      }
      return res;
    };
  },

  // WARNING: subscriptions now don't wait for auth.
  // This allows to return channel instead of a promise
  // which avoids unnecessary complication of code that needs channel object to unsubscribe/change_params.
  subscriptionWrap: function(action, debugTag) {
    var authorizer = this;
    return function() {
      var subject = this;
      var argumentsArr = Array.from(arguments);

      if (authorizer.authSucceeded) {
        return action.apply(subject, argumentsArr);
      } else if (authorizer.authSucceeded === undefined) {
        console.error(debugTag + ": subscription called before authorization " + (!!authorizer.authPromise ? 'happened' : 'even started'))
      } else {
        console.warn(debugTag + ": ignoring subscription after failed authorization")
      }
    };
  },

  wrapAll: function() {
    var authorizer = this;
    Array.from(arguments).forEach((descr) => {
      var klass = descr[0];
      klass.authorizerFailSkipper = authorizer.failSkipper;
      descr[1].forEach((action) => {
        klass[action] = authorizer.wrap(klass[action], descr[2], descr[3], klass.klassName + '.' + action);
      });

      if (klass.subscribe) {
        klass.cableUrl = authorizer.params.api_url;
        var oldSubscriber = klass.subscribe;
        klass.subscribe = authorizer.subscriptionWrap(oldSubscriber, klass.klassName + '.subscribe');
      }
    });
    return;
  }
};

export default PassportAuth;

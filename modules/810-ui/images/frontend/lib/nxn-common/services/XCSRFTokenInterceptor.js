import XCSRFTokens from './XCSRFTokens.js';

axios.interceptors.request.use(function (config) {
  if (!config.withoutCsrfToken) {
    if (!config.params) config.params = {};

    var token;
    var api_urls = Object.keys(XCSRFTokens).sort(function(a, b){ return b.length - a.length; });
    for (var i = 0; i < api_urls.length; i++) {
      if (config.url.indexOf(api_urls[i]) == 0) {
        token = XCSRFTokens[api_urls[i]];
        break;
      }
    }
    if (token) {
      config.headers['X-CSRF-Token'] = token;
    }
  }
  return config;
});

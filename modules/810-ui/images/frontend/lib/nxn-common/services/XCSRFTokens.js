function XCSRFTokensLoader() {
  var res = {};
  var localTokenEl = document.getElementsByName('csrf-token')[0];
  if (localTokenEl) {
    res[`${window.location.protocol}//${document.location.hostname}`] = localTokenEl.getAttribute('content');
  }
  return res;
}

var XCSRFTokens;
export default XCSRFTokens = XCSRFTokensLoader();

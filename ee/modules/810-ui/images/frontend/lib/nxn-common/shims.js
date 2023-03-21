Object.assignedOnly = function(src, keys) {
  return Object.assign(
    {},
    keys.reduce(function(acc, k) { acc[k] = src[k]; return acc; }, {})
  );
};

Object.copyExcept = function(src, excludedKeys) {
  return Object.assignedOnly(
    src,
    Object.keys(src).filter(function(key){ return excludedKeys.indexOf(key) < 0; })
  );
};

window.equalInSome = function(a, b, keys) {
  return deepEqual(Object.assignedOnly(a, keys), Object.assignedOnly(b, keys));
};

Object.values = Object.values || (function(obj){ return Object.keys(obj).map(function(key){ return obj[key]; }); });

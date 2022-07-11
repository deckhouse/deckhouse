module.exports.dumpError = (error) => {
  let result = '';
  try {
    // Use simplifyObj to prevent huge dump of circular structures.
    result = JSON.stringify(simplifyObj({obj: error, depth: 7}))
  } catch (e) {
    result = `${error.name||'UnknownError'}: ${error.message||'no message'}`
  }
  return result
}

// Use simplifyObj to break circular structures.
// It copies objects recursively until depth is reached.
const simplifyObj = ({obj, depth}) => {
  if (depth <= 0 ) {
    return '[Too deep]';
  }
  if (typeof(obj) == 'function'){
    return '[Function]';
  }
  // Object or Array.
  if (obj !== null && typeof(obj) == 'object') {
    let simpleObj = {}
    for (let prop in obj ){
      if (obj.hasOwnProperty(prop)){
        simpleObj[prop] = simplifyObj({obj: obj[prop], depth: depth - 1})
      }
    }
    return simpleObj;
  }
  return obj;
}

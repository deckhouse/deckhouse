module.exports.dumpError = (error) => {
  let result = '';
  try {
    // Max 10 recursive calls
    result = JSON.stringify(simplifyObj({error, depth: 7}))
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
  if (typeof(obj) == 'object') {
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

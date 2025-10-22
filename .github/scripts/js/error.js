// Copyright 2022 Flant JSC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

export const jsonToHash = function(obj) {
  // The simplest solution for "flat" objects
  let pairs = [];
  for (let p in obj) {
    if (obj.hasOwnProperty(p)) {
      pairs.push(encodeURIComponent(p) + "=" + encodeURIComponent(obj[p]));
    }
  }
  return pairs.join("&");
}

export const jsonFromHash = function(hash) {
  // The simplest solution from https://stackoverflow.com/a/8486188
  let result = {};
  hash.split("&").forEach(function(part) {
    let item = part.split("=");
    result[item[0]] = decodeURIComponent(item[1]);
  });
  return result;
}



export const secondsToHumanReadable = function(seconds) {
  if (!seconds) {
    return "";
  }
  if (seconds < 60) {
    return `${seconds}s`;
  }
  if (seconds < 60 * 60 ) {
    return `${Math.floor(seconds/60)}m ${seconds%60}s`.replace(/ 0\w+/g, '');
  }
  let hourSec = 60 * 60;
  let daySec = 24 * hourSec
  if (seconds < daySec ) {
    let hour = Math.floor(seconds/hourSec)
    let min = Math.floor((seconds%hourSec) / 60)
    let sec = Math.floor((seconds%hourSec) % 60)
    return `${hour}h ${min}m ${sec}s`.replace(/ 0\w+/g, '');
  }
  let monthSec = 30 * daySec;
  if (seconds < monthSec ) {
    let day = Math.floor(seconds/daySec)
    let remSec = seconds % daySec;
    let hour = Math.floor(remSec/hourSec)
    let min = Math.floor((remSec%hourSec) / 60)
    let sec = Math.floor((remSec%hourSec) % 60)
    return `${day}day ${hour}h ${min}m ${sec}s`.replace(/ 0\w+/g, '');
  }
  let yearSec = 365 * daySec
  if (seconds < yearSec ) {
    let month = Math.floor(seconds/monthSec)
    let day = Math.floor((seconds % monthSec)/daySec)
    let remSec = ((seconds % monthSec)% daySec);
    let hour = Math.floor(remSec/hourSec)
    let min = Math.floor((remSec%hourSec) / 60)
    let sec = Math.floor((remSec%hourSec) % 60)
    return `${month}mon ${day}day ${hour}h ${min}m ${sec}s`.replace(/ 0\w+/g, '');
  }
  let year = Math.floor(seconds/yearSec)
  let month = Math.floor((seconds%yearSec)/monthSec)
  let day = Math.floor(((seconds%yearSec)%monthSec)/daySec)
  let remSec = ((seconds%yearSec)%monthSec)%daySec;
  let hour = Math.floor(remSec/hourSec)
  let min = Math.floor((remSec%hourSec) / 60)
  let sec = Math.floor((remSec%hourSec) % 60)
  return `${year}y ${month}mon ${day}day ${hour}h ${min}m ${sec}s`.replace(/ 0\w+/g, '');
}

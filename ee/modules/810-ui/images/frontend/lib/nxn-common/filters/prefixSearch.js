// http://codereview.stackexchange.com/a/7025 adapted for arrays
function combinations(arr) {
  var fn;
  fn = function(active, rest, a) {
    if (active.length < 1 && rest.length < 1) return;
    if (rest.length < 1) {
      a.push(active);
    } else {
      fn(active.concat(rest[0]), rest.slice(1), a);
      fn(active, rest.slice(1), a);
    }
    return a;
  };
  return fn([], arr, []);
}

// ['1', '2', '3'] => ['1', '2', '3', '1-2', '1-3', '2-3', '1-2-3', ....]
function toBlocks(str) {
  var words = str.toLowerCase().split(/ |\.|-|_/).filter(function(word) { return word.length > 0; });
  var blocks = [];

  var comb_args = []; // TODO: [0..words.length-1];
  combinations(comb_args).forEach(function(indexes) {
    var slice = indexes.map(function(i) { return words[i]; });
    if (indexes.length > 1) {
      ['.', '-', '_', ''].forEach(function(joiner) {
        blocks.push({ matcher: slice.join(joiner), wordsIdxs: indexes });
      });
    } else {
      if (indexes[0] === words.length - 1) {
        return blocks.push({ matcher: slice[0], wordsIdxs: indexes });
      } else {
        ['.', '-', '_'].forEach(function(joiner) {
          blocks.push({ matcher: slice[0] + joiner, wordsIdxs: indexes });
        });
      }
    }
  });
  return blocks;
}

var $parse = function() {}; // TODO: $parse

export default function(collection, props, query) {
  if (!query) return [];
  var queryWords = query.toLowerCase().split(' ').filter(function(word) { return word.length > 0; });

  return collection.filter(function(item) {
    if (!item.prefixSearchBlocks) {
      item.prefixSearchBlocks = {};
      props.forEach(function(prop) {
        let val = $parse(prop)(item);
        if (typeof val == 'string') item.prefixSearchBlocks[prop] = toBlocks(val);
      });
    }

    return props.some(function(prop) {
      var blocks = Object.assign([], item.prefixSearchBlocks[prop]);
      if (!blocks) return false;

      var alreadyHitWordsIdxs = [];
      return queryWords.every(function(queryWord) {
        var hits = blocks.filter(function(block){
          return block.matcher.startsWith(queryWord) && block.wordsIdxs.every(function(i) { return alreadyHitWordsIdxs.indexOf(i) < 0; });
        });
        if (hits.length > 0) {
          var mostFittingHit = hits[0];
          var minDist = mostFittingHit.matcher.length;
          hits.forEach(function(hit) {
            if ((hit.matcher.length - queryWord.length) < minDist) {
              mostFittingHit = hit;
            }
          });
          alreadyHitWordsIdxs = alreadyHitWordsIdxs.concat(mostFittingHit.wordsIdxs);
          return true;
        } else {
          return false;
        }
      });
    });
  });
}



function Dataset() {
  this.data = [];
}

Dataset.prototype.clear = function() {
  this.data = [];
}
Dataset.prototype.length = function() {
  return this.data.length;
}
Dataset.prototype.push = function(item) {
    this.data.push(item);
}
Dataset.prototype.forEach = function(fn){
  this.data.forEach(fn);
}
Dataset.prototype.get = function(i){
  return this.data[i];
}

export let dataset = new Dataset();

// TODO: cleaner implementation and use of mixins in nxn?
export default function nxnEditableStaleState(Resource) {
  Resource.staleOnFailOfActions = function(actionNames) {
    actionNames.forEach(function(actionName) {
      Resource.prototype[actionName] = Resource.staleOnFailOfActionWrapper(Resource.prototype[actionName]);
    });
  };

  Resource.staleOnFailOfAction = function(actionName) {
    Resource.prototype[actionName] = Resource.staleOnFailOfActionWrapper(Resource.prototype[actionName]);
  };

  Resource.staleOnFailOfActionWrapper = function(action) {
    return function() {
      var self = this;
      return action.apply(this, arguments).catch((error) =>{
        self.$$stale();
        return Promise.reject(error);
      });
    };
  };

  Resource.prototype.$$stale = function() {
    this.is_stale = true;
  };

  return Resource;
}

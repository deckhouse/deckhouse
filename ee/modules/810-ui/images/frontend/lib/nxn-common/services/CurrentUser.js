// TODO: raise smth if it's not in sessionStorage
// TODO: reload value from sessionStorage
var CurrentUser = {
  klassName: 'User',
  loaded: false,

  init: function(data) {
    var params = (typeof data === 'string') ? JSON.parse(data) : data;
    Object.assign(this, params);
    if (!this.manages) this.manages = {};
    if (!this.manages.team_ids) this.manages.team_ids = [];
    this.is_manager = this.manages.team_ids.length > 0;
    this.loaded = true;
  },

  canAsContact: function(permission, project_uuid) {
    var credentials = this.can_impersonate ? this.impersonated_credentials : this.credentials;
    if (!credentials) return false;

    if (typeof(project_uuid) !== 'undefined') {
      return credentials[project_uuid] && credentials[project_uuid].includes(permission);
    } else {
      var permissions = Object.values(credentials);
      permissions = permissions.reduce(function(a, b) { return a.concat(b); }, []);
      return permissions.includes(permission);
    }
  },

  to_short_form: function() {
    return { uuid: this.uuid, email: this.email, display_name: this.display_name };
  }
};

var json_data = sessionStorage.getItem('current_user');
if (json_data) CurrentUser.init(json_data);

if (CurrentUser.can_impersonate) {
  CurrentUser.impersonated_uuid = undefined;
  CurrentUser.impersonated_name = undefined;
  CurrentUser.impersonated_credentials = {};
  CurrentUser.impersonated_allowed_locales = undefined;
}

export default CurrentUser;

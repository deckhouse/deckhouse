import NxnDB from 'nxn-common/services/NxnDB.js';
import NxnResource from 'nxn-common/services/NxnResource.js';

var Routes = [
  `${window.location.protocol}//:hostname/api/edit_sessions`,
  { hostname: document.location.hostname },
  {
    query: { method: 'GET', storeResponse: true, format: 'array', withCredentials: true }
  },
  {},
  'EditSessionsChannel'
];

export default class EditSession extends NxnResource {
  static startHeartbeat(channel) {
    var Resource = this;
    return setInterval(function(){
      (channel || Resource.channel).perform('edit_session_heartbeat');
    }, 10000);
  }
}

NxnDB(EditSession, 'EditSession');
EditSession.setRoutes.apply(EditSession, Routes);

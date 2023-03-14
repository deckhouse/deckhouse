import GlobalNxnFlash from './GlobalNxnFlash.js';
import FormatError from './FormatError.js';

var AuthSessionAutoRefresher = {
  ping: function(url) {
    // special path to auto-prolong session
    return axios.get(url, { withCredentials: true, ignoreProgressBar: true }).then(
      (resp) => {
        if (resp.data && resp.data.revision_mismatch) {
          GlobalNxnFlash.proposeReload("Reload is required to update this app.");
        }
      },
      (error) => {
        this.stop(error);
      }
    );
  },

  start: function(url) {
    if (!url) {
      url = '/session_status';
    }
    this.stopSwitch = setInterval(
      () => {
        this.ping(url).catch((error) => {
          GlobalNxnFlash.show('error', `AuthSessionAutoRefresher failed: ${FormatError(error)}`, 0, 'AuthSessionAutoRefresher');
        });
      },
      60000, 0, true // 1.minute
    );
    return this.stopSwitch;
  },

  stop: (resp) => {
    if (this.stopSwitch) {
      clearInterval(this.stopSwitch);
      this.stopSwitch = undefined;
      Promise.reject(resp);
    }
    return;
  }
};

export default AuthSessionAutoRefresher;

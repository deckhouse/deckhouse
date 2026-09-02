// DVP GS step 4: install orchestration and config.yml token pass (requires constants + getting-started-dvp.js).
// Uses generate_password / update_license_parameters from getting-started.js (DKP).

// EE license token for <YOUR_ACCESS_STRING_IS_HERE> in DVP config.yml.
function dvpLicenseToken() {
  if (typeof $ === 'undefined' || !$.cookie) {
    return '';
  }
  return $.cookie('license-token') || $.cookie('demotoken') || '';
}

function dvpInstallSessionValues() {
  return dvpSessionValues({ licenseCfg: dvpLicenseToken() });
}

// Config.yml blocks rendered inside .dvp-config-yaml (INSTALL_CONFIG.liquid).
function findConfigCodeElements() {
  return Array.prototype.slice.call(document.querySelectorAll('.dvp-config-yaml code'));
}

function ensureConfigBaselines() {
  findConfigCodeElements().forEach(function (codeEl) {
    gsEnsureSnapshotBaseline(codeEl, GS_SNAPSHOT_ATTR);
  });
}

function refreshConfigFromBaseline(codeEl) {
  if (!codeEl) {
    return;
  }
  dvpApplyTokensToElement(codeEl, dvpPlaceholderRules(dvpInstallSessionValues(), 'config'), GS_SNAPSHOT_ATTR);
}

function refreshAllConfigFromBaseline() {
  findConfigCodeElements().forEach(refreshConfigFromBaseline);
}

function refreshVisibleSnippets() {
  refreshAllConfigFromBaseline();
  if (typeof dvp_refresh_command_placeholders === 'function') {
    dvp_refresh_command_placeholders();
  }
  if (typeof dvp_config_highlight === 'function') {
    dvp_config_highlight();
  }
}

function dvp_config_update() {
  ensureConfigBaselines();
  refreshVisibleSnippets();
}

// B: refresh DVP config.yml after DKP update_license_parameters.
function dvpWrapUpdateLicenseParameters() {
  if (typeof update_license_parameters !== 'function' || update_license_parameters._dvpWrapped) {
    return;
  }
  var original = update_license_parameters;
  update_license_parameters = function (newtoken) {
    original.apply(this, arguments);
    dvp_config_update();
  };
  update_license_parameters._dvpWrapped = true;
}

dvpWrapUpdateLicenseParameters();

function dvp_run_install_page_init() {
  var init = Promise.resolve();
  if (typeof generate_caps_ssh_key === 'function') {
    init = generate_caps_ssh_key(false);
  }
  return init
    .then(function () {
      if (typeof generate_password === 'function') {
        generate_password(false);
      }
      if (typeof update_license_parameters === 'function') {
        update_license_parameters();
        if (!update_license_parameters._dvpWrapped) {
          dvp_config_update();
        }
      } else {
        dvp_config_update();
      }
    })
    .catch(function (err) {
      console.error('DVP install init failed:', err);
    });
}

document.addEventListener('DOMContentLoaded', function () {
  dvp_run_install_page_init();
});

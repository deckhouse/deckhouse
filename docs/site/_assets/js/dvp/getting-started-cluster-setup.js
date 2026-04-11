/**
 * DVP getting started — cluster parameters form (step 2).
 * Depends: jQuery, sessionStorage, optional dcodeIO.bcrypt (for admin password hash).
 */

var DVP_STORAGE_KEYS = {
  internal: 'dvp-internal-network-cidrs',
  vm: 'dvp-virtual-machine-cidrs',
  pod: 'dvp-pod-subnet-cidr',
  service: 'dvp-service-subnet-cidr',
  worker: 'dvp-worker-node-ip',
  nfsShare: 'dvp-nfs-share',
  nfsHost: 'dvp-nfs-host',
  ssh: 'dvp-caps-ssh-public-key',
  adminUser: 'dvp-admin-username',
  adminHash: 'dvp-admin-password-hash',
  nfsScName: 'dvp-nfs-storage-class-name',
  dvcrSize: 'dvp-dvcr-storage-size',
  projectName: 'dvp-project-name',
};

var DVP_DEFAULTS = {
  pod: '10.115.0.0/16',
  service: '10.225.0.0/16',
  domain: '%s.domain.my',
  nfsShare: '/srv/nfs/dvp',
  nfsHost: '192.168.1.100',
  nfsStorageClass: 'nfs-storage-class',
  dvcrPvcSize: '50G',
  projectName: 'test-project',
};

var DVP_PUBLIC_DOMAIN_PATTERN = /^(%s([-a-z0-9]*[a-z0-9])?|[a-z0-9]([-a-z0-9]*)?%s([-a-z0-9]*)?[a-z0-9]|[a-z0-9]([-a-z0-9]*)?%s)(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/;

var DVP_SNAPSHOT_ATTR = 'data-dvp-snippet-snapshot';
var DVP_INSTALL_SNAPSHOT_ATTR = 'data-dvp-install-snapshot';

function dvp_debounce(fn, ms) {
  var t;
  return function () {
    var ctx = this;
    var args = arguments;
    clearTimeout(t);
    t = setTimeout(function () {
      fn.apply(ctx, args);
    }, ms);
  };
}

function dvp_parse_ipv4(s) {
  var m = String(s).trim().match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
  if (!m) return null;
  var p = [1, 2, 3, 4].map(function (i) {
    return parseInt(m[i], 10);
  });
  if (p.some(function (x) {
    return x > 255 || x < 0;
  })) return null;
  /* Bitwise (<<24) breaks addresses like 192.168.x.x (signed 32-bit); use arithmetic. */
  return (p[0] * 16777216 + p[1] * 65536 + p[2] * 256 + p[3]) >>> 0;
}

function dvp_parse_ipv4_cidr(str) {
  var s = String(str).trim();
  var m = s.match(/^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\/(\d{1,2})$/);
  if (!m) return null;
  var pref = parseInt(m[2], 10);
  if (pref < 0 || pref > 32) return null;
  var ip = dvp_parse_ipv4(m[1]);
  if (ip === null) return null;
  var mask = pref === 0 ? 0 : (~((Math.pow(2, 32 - pref) - 1) >>> 0)) >>> 0;
  var network = (ip & mask) >>> 0;
  var size = pref === 32 ? 1 : Math.pow(2, 32 - pref);
  var end = (network + size - 1) >>> 0;
  return { start: network, end: end, raw: s };
}

function dvp_cidrs_overlap(a, b) {
  var A = dvp_parse_ipv4_cidr(a);
  var B = dvp_parse_ipv4_cidr(b);
  if (!A || !B) return false;
  return !(A.end < B.start || B.end < A.start);
}

function dvp_cidr_looks_complete(s) {
  return /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\/\d{1,2}$/.test(String(s).trim());
}

function dvp_effective_domain() {
  var v = ($('#clusterdomain').val() || '').trim();
  return v || DVP_DEFAULTS.domain;
}

function dvp_effective_nfs_share() {
  var v = ($('#nfsshare').val() || '').trim();
  return v || DVP_DEFAULTS.nfsShare;
}

function dvp_effective_nfs_host() {
  var v = ($('#nfshost').val() || '').trim();
  return v || DVP_DEFAULTS.nfsHost;
}

function dvp_valid_nfs_host(s) {
  if (!s) return false;
  if (dvp_parse_ipv4(s) !== null) return true;
  return /^[a-zA-Z0-9]([a-zA-Z0-9_.-]*[a-zA-Z0-9])?$/.test(s);
}

/** RFC 1123 DNS label for StorageClass / project names (lowercase). */
function dvp_valid_k8s_dns_subdomain(s) {
  var t = String(s).trim();
  if (!t || t.length > 63) return false;
  return /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(t);
}

/** Kubernetes-style quantity, e.g. 50G, 10Gi. */
function dvp_valid_resource_quantity(s) {
  return /^[0-9]+(\.[0-9]+)?([EePpTtGgMmKk]i|[EePpTtGgMmKk])$/i.test(String(s).trim());
}

function dvp_update_node(selector, storageKey) {
  var v = sessionStorage.getItem(storageKey);
  if (v && v.length > 0) {
    $(selector).val(v);
  }
}

function dvp_restore_data() {
  if (!$('#internalnetworkcidrs').length) return;

  dvp_update_node('#internalnetworkcidrs', DVP_STORAGE_KEYS.internal);
  dvp_update_node('#virtualmachinecidrs', DVP_STORAGE_KEYS.vm);
  dvp_update_node('#podsubnetcidr', DVP_STORAGE_KEYS.pod);
  dvp_update_node('#servicesubnetcidr', DVP_STORAGE_KEYS.service);
  dvp_update_node('#workernodeip', DVP_STORAGE_KEYS.worker);
  dvp_update_node('#clusterdomain', 'dhctl-domain');
  dvp_update_node('#nfsshare', DVP_STORAGE_KEYS.nfsShare);
  dvp_update_node('#nfshost', DVP_STORAGE_KEYS.nfsHost);
  dvp_update_node('#capssshkey', DVP_STORAGE_KEYS.ssh);
  dvp_update_node('#adminusername', DVP_STORAGE_KEYS.adminUser);
  dvp_update_node('#nfsstoragclassname', DVP_STORAGE_KEYS.nfsScName);
  dvp_update_node('#dvcrdisksize', DVP_STORAGE_KEYS.dvcrSize);
  dvp_update_node('#projectname', DVP_STORAGE_KEYS.projectName);

  if (!sessionStorage.getItem(DVP_STORAGE_KEYS.pod)) {
    $('#podsubnetcidr').attr('placeholder', DVP_DEFAULTS.pod);
  }
  if (!sessionStorage.getItem(DVP_STORAGE_KEYS.service)) {
    $('#servicesubnetcidr').attr('placeholder', DVP_DEFAULTS.service);
  }
  if (!sessionStorage.getItem('dhctl-domain')) {
    $('#clusterdomain').attr('placeholder', DVP_DEFAULTS.domain);
  }
  if (!sessionStorage.getItem(DVP_STORAGE_KEYS.nfsShare)) {
    $('#nfsshare').attr('placeholder', DVP_DEFAULTS.nfsShare);
  }
  if (!sessionStorage.getItem(DVP_STORAGE_KEYS.nfsHost)) {
    $('#nfshost').attr('placeholder', DVP_DEFAULTS.nfsHost);
  }
  if (!sessionStorage.getItem(DVP_STORAGE_KEYS.nfsScName) && $('#nfsstoragclassname').length) {
    $('#nfsstoragclassname').attr('placeholder', DVP_DEFAULTS.nfsStorageClass);
  }
  if (!sessionStorage.getItem(DVP_STORAGE_KEYS.dvcrSize) && $('#dvcrdisksize').length) {
    $('#dvcrdisksize').attr('placeholder', DVP_DEFAULTS.dvcrPvcSize);
  }
  if (!sessionStorage.getItem(DVP_STORAGE_KEYS.projectName) && $('#projectname').length) {
    $('#projectname').attr('placeholder', DVP_DEFAULTS.projectName);
  }
}

function dvp_clear_field_errors($input) {
  var $wrap = $input.closest('.form__row').length ? $input.closest('.form__row') : $input.parent();
  $input.removeClass('invalid');
  $input.removeAttr('aria-invalid');
  $wrap.find('.invalid-message-main').removeClass('active');
  $wrap.find('.invalid-message-example-com').removeClass('active');
  $wrap.find('.invalid-message-overlap').removeClass('active');
}

function dvp_set_field_error($input, kind) {
  var $wrap = $input.closest('.form__row').length ? $input.closest('.form__row') : $input.parent();
  $input.addClass('invalid');
  $input.attr('aria-invalid', 'true');
  if (kind === 'example') {
    $wrap.find('.invalid-message-example-com').addClass('active');
  } else if (kind === 'overlap') {
    $wrap.find('.invalid-message-overlap').addClass('active');
  } else {
    $wrap.find('.invalid-message-main').addClass('active');
  }
}

/** Показывать подсветку ошибки: всегда для пересечения CIDR; иначе только после взаимодействия с полем или по запросу «Далее». */
function dvp_maybe_set_field_error($input, kind, options) {
  if (!$input || !$input.length) return;
  if (kind === 'overlap') {
    dvp_set_field_error($input, kind);
    return;
  }
  if (options && options.showAllErrors) {
    dvp_set_field_error($input, kind);
    return;
  }
  if ($input.data('dvpTouched')) {
    dvp_set_field_error($input, kind);
  }
}

function dvp_generate_admin_password_hash(password) {
  if (typeof dcodeIO === 'undefined' || !dcodeIO.bcrypt) return null;
  var bcrypt = dcodeIO.bcrypt;
  var salt = bcrypt.genSaltSync(10);
  var hash = bcrypt.hashSync(password, salt);
  return btoa(hash);
}

function dvp_sync_password_hash() {
  var pwd = ($('#adminpassword').val() || '');
  if (pwd.length >= 8) {
    var h = dvp_generate_admin_password_hash(pwd);
    if (h) {
      sessionStorage.setItem(DVP_STORAGE_KEYS.adminHash, h);
    }
  } else {
    sessionStorage.removeItem(DVP_STORAGE_KEYS.adminHash);
  }
}

function dvp_validate_cluster_form(options) {
  options = options || {};
  var strictCidr = options.strictCidr === true;

  function maybeErr($el, k) {
    dvp_maybe_set_field_error($el, k, options);
  }

  var ids = ['#internalnetworkcidrs', '#virtualmachinecidrs', '#podsubnetcidr', '#servicesubnetcidr', '#workernodeip', '#clusterdomain', '#nfsshare', '#nfshost', '#nfsstoragclassname', '#dvcrdisksize', '#projectname', '#capssshkey', '#adminusername', '#adminpassword'];
  ids.forEach(function (sel) {
    var $el = $(sel);
    if ($el.length) dvp_clear_field_errors($el);
  });
  $('#dvp-overlap-banner').removeClass('active');

  var ok = true;
  var internal = ($('#internalnetworkcidrs').val() || '').trim();
  var vm = ($('#virtualmachinecidrs').val() || '').trim();
  var podInput = ($('#podsubnetcidr').val() || '').trim();
  var svcInput = ($('#servicesubnetcidr').val() || '').trim();
  var worker = ($('#workernodeip').val() || '').trim();
  var domain = dvp_effective_domain();
  var nfsPath = dvp_effective_nfs_share();
  var nfsHost = dvp_effective_nfs_host();
  var ssh = ($('#capssshkey').val() || '').trim();
  var admin = ($('#adminusername').val() || '').trim();
  var pwd = ($('#adminpassword').val() || '');
  var nfsScInput = ($('#nfsstoragclassname').length ? ($('#nfsstoragclassname').val() || '').trim() : '');
  var dvcrIn = ($('#dvcrdisksize').length ? ($('#dvcrdisksize').val() || '').trim() : '');
  var projIn = ($('#projectname').length ? ($('#projectname').val() || '').trim() : '');

  function validateCidrField($field, val, required) {
    if (!val) {
      if (required) {
        maybeErr($field, 'main');
        ok = false;
      }
      return false;
    }
    if (!dvp_cidr_looks_complete(val)) {
      if (strictCidr) {
        maybeErr($field, 'main');
        ok = false;
      } else if (required) {
        ok = false;
      } else if (val.length > 0) {
        ok = false;
      }
      return false;
    }
    if (!dvp_parse_ipv4_cidr(val)) {
      maybeErr($field, 'main');
      ok = false;
      return false;
    }
    return true;
  }

  var internalValid = validateCidrField($('#internalnetworkcidrs'), internal, true);
  var vmValid = validateCidrField($('#virtualmachinecidrs'), vm, true);

  var podEffective = podInput || DVP_DEFAULTS.pod;
  var podValid = true;
  if (podInput) {
    podValid = validateCidrField($('#podsubnetcidr'), podInput, false);
  } else if (!dvp_parse_ipv4_cidr(podEffective)) {
    ok = false;
    podValid = false;
  }

  var svcEffective = svcInput || DVP_DEFAULTS.service;
  var svcValid = true;
  if (svcInput) {
    svcValid = validateCidrField($('#servicesubnetcidr'), svcInput, false);
  } else if (!dvp_parse_ipv4_cidr(svcEffective)) {
    ok = false;
    svcValid = false;
  }

  if (!worker) {
    maybeErr($('#workernodeip'), 'main');
    ok = false;
  } else if (dvp_parse_ipv4(worker) === null) {
    maybeErr($('#workernodeip'), 'main');
    ok = false;
  } else if (internalValid && dvp_parse_ipv4_cidr(internal)) {
    if (!dvp_ipv4_in_cidr(worker, internal)) {
      maybeErr($('#workernodeip'), 'main');
      ok = false;
    }
  }

  if (!domain.match(DVP_PUBLIC_DOMAIN_PATTERN)) {
    maybeErr($('#clusterdomain'), 'main');
    ok = false;
  } else if (domain.match(/\.example\.com/)) {
    maybeErr($('#clusterdomain'), 'example');
    ok = false;
  }

  if (!/^\/[A-Za-z0-9_\-\/]*$/.test(nfsPath)) {
    var nfsShareInput = ($('#nfsshare').val() || '').trim();
    if (nfsShareInput.length > 0 || options.showAllErrors) {
      maybeErr($('#nfsshare'), 'main');
    }
    ok = false;
  }

  if (!dvp_valid_nfs_host(nfsHost)) {
    var nfsHostInput = ($('#nfshost').val() || '').trim();
    if (nfsHostInput.length > 0 || options.showAllErrors) {
      maybeErr($('#nfshost'), 'main');
    }
    ok = false;
  }

  if (!ssh) {
    maybeErr($('#capssshkey'), 'main');
    ok = false;
  } else if (!/^(ssh-rsa|ssh-ed25519|ecdsa-sha2-|sk-ssh-ed25519|sk-ecdsa-sha2-)/.test(ssh.trim())) {
    maybeErr($('#capssshkey'), 'main');
    ok = false;
  }

  if (!admin) {
    maybeErr($('#adminusername'), 'main');
    ok = false;
  }

  if (!pwd || pwd.length < 8) {
    maybeErr($('#adminpassword'), 'main');
    ok = false;
  } else {
    dvp_sync_password_hash();
    if (!sessionStorage.getItem(DVP_STORAGE_KEYS.adminHash)) {
      maybeErr($('#adminpassword'), 'main');
      ok = false;
    }
  }

  if ($('#nfsstoragclassname').length && nfsScInput && !dvp_valid_k8s_dns_subdomain(nfsScInput)) {
    maybeErr($('#nfsstoragclassname'), 'main');
    ok = false;
  }
  if ($('#dvcrdisksize').length && dvcrIn && !dvp_valid_resource_quantity(dvcrIn)) {
    maybeErr($('#dvcrdisksize'), 'main');
    ok = false;
  }
  if ($('#projectname').length && projIn && !dvp_valid_k8s_dns_subdomain(projIn)) {
    maybeErr($('#projectname'), 'main');
    ok = false;
  }

  var cidrList = [];
  if (internalValid) cidrList.push({ id: '#internalnetworkcidrs', c: internal });
  if (vmValid) cidrList.push({ id: '#virtualmachinecidrs', c: vm });
  if (podValid) cidrList.push({ id: '#podsubnetcidr', c: podEffective });
  if (svcValid) cidrList.push({ id: '#servicesubnetcidr', c: svcEffective });

  if (cidrList.length >= 2) {
    for (var i = 0; i < cidrList.length; i++) {
      for (var j = i + 1; j < cidrList.length; j++) {
        if (dvp_cidrs_overlap(cidrList[i].c, cidrList[j].c)) {
          dvp_maybe_set_field_error($(cidrList[i].id), 'overlap', options);
          dvp_maybe_set_field_error($(cidrList[j].id), 'overlap', options);
          $('#dvp-overlap-banner').addClass('active');
          ok = false;
        }
      }
    }
  }

  if (ok && !options.skipPersist) {
    sessionStorage.setItem(DVP_STORAGE_KEYS.internal, internal);
    sessionStorage.setItem(DVP_STORAGE_KEYS.vm, vm);
    if (podInput) {
      sessionStorage.setItem(DVP_STORAGE_KEYS.pod, podInput);
    } else {
      sessionStorage.removeItem(DVP_STORAGE_KEYS.pod);
    }
    if (svcInput) {
      sessionStorage.setItem(DVP_STORAGE_KEYS.service, svcInput);
    } else {
      sessionStorage.removeItem(DVP_STORAGE_KEYS.service);
    }
    sessionStorage.setItem(DVP_STORAGE_KEYS.worker, worker);
    sessionStorage.setItem('dhctl-domain', domain);
    sessionStorage.setItem(DVP_STORAGE_KEYS.nfsShare, ($('#nfsshare').val() || '').trim() || DVP_DEFAULTS.nfsShare);
    sessionStorage.setItem(DVP_STORAGE_KEYS.nfsHost, ($('#nfshost').val() || '').trim() || DVP_DEFAULTS.nfsHost);
    sessionStorage.setItem(DVP_STORAGE_KEYS.ssh, ssh);
    sessionStorage.setItem('dhctl-sshkey', ssh);
    sessionStorage.setItem(DVP_STORAGE_KEYS.adminUser, admin);
    if ($('#nfsstoragclassname').length) {
      if (nfsScInput) {
        sessionStorage.setItem(DVP_STORAGE_KEYS.nfsScName, nfsScInput);
      } else {
        sessionStorage.removeItem(DVP_STORAGE_KEYS.nfsScName);
      }
    }
    if ($('#dvcrdisksize').length) {
      if (dvcrIn) {
        sessionStorage.setItem(DVP_STORAGE_KEYS.dvcrSize, dvcrIn);
      } else {
        sessionStorage.removeItem(DVP_STORAGE_KEYS.dvcrSize);
      }
    }
    if ($('#projectname').length) {
      if (projIn) {
        sessionStorage.setItem(DVP_STORAGE_KEYS.projectName, projIn);
      } else {
        sessionStorage.removeItem(DVP_STORAGE_KEYS.projectName);
      }
    }
  }

  dvp_toggle_next_step_enabled(ok);
  if (ok && typeof dvp_config_update === 'function' && !options.skipConfigUpdate) {
    dvp_config_update();
  }

  return ok;
}

function dvp_ipv4_in_cidr(ip, cidr) {
  var n = dvp_parse_ipv4(ip);
  var c = dvp_parse_ipv4_cidr(cidr);
  if (n === null || !c) return false;
  return n >= c.start && n <= c.end;
}

/** Plain config-yml raw nodes (vanilla DOM; avoids relying only on jQuery). */
function dvp_get_config_yml_raw_elements() {
  var list = [];
  var seen = new Set();
  function add(el) {
    if (el && !seen.has(el)) {
      seen.add(el);
      list.push(el);
    }
  }
  try {
    document.querySelectorAll('[config-yml]').forEach(add);
  } catch (e) {}
  if (!list.length) {
    document.querySelectorAll('.snippetcut__raw[data-snippetcut-text]').forEach(function (el) {
      var tx = el.textContent || '';
      var looksLikeDvpConfig =
        (tx.indexOf('kind: ClusterConfiguration') !== -1 && (tx.indexOf('<POD_SUBNET_CIDR>') !== -1 || tx.indexOf('<USER_NAME>') !== -1)) ||
        (tx.indexOf('kind: InitConfiguration') !== -1 && tx.indexOf('<PUBLIC_DOMAIN_TEMPLATE>') !== -1) ||
        (tx.indexOf('kind: InitConfiguration') !== -1 && tx.indexOf('clusterType: Static') !== -1 && tx.indexOf('registry.deckhouse.ru/deckhouse') !== -1) ||
        tx.indexOf('<DVCR_STORAGE_SIZE>') !== -1 ||
        tx.indexOf('<NFS_STORAGE_CLASS_NAME>') !== -1;
      if (looksLikeDvpConfig) {
        add(el);
      }
    });
  }
  return list;
}

function dvp_ensure_snippet_snapshots() {
  dvp_get_config_yml_raw_elements().forEach(function (el) {
    if (el._dvpYamlSnapshot !== undefined) return;
    var fromAttr = el.getAttribute(DVP_SNAPSHOT_ATTR);
    if (fromAttr != null && fromAttr !== '') {
      el._dvpYamlSnapshot = fromAttr;
    } else {
      el._dvpYamlSnapshot = el.textContent;
    }
  });
}

/** Keep template tokens when there is no stored value (e.g. user opened install before step 2). */
function dvp_split_join_if_nonempty(t, token, val) {
  if (val === null || val === undefined) return t;
  var s = String(val);
  if (s === '') return t;
  return t.split(token).join(s);
}

function dvp_apply_placeholders_to_text(base) {
  var internal = sessionStorage.getItem(DVP_STORAGE_KEYS.internal) || '';
  var vm = sessionStorage.getItem(DVP_STORAGE_KEYS.vm) || '';
  var pod = sessionStorage.getItem(DVP_STORAGE_KEYS.pod) || DVP_DEFAULTS.pod;
  var svc = sessionStorage.getItem(DVP_STORAGE_KEYS.service) || DVP_DEFAULTS.service;
  var worker = sessionStorage.getItem(DVP_STORAGE_KEYS.worker) || '';
  var nfsH = sessionStorage.getItem(DVP_STORAGE_KEYS.nfsHost) || DVP_DEFAULTS.nfsHost;
  var nfsS = sessionStorage.getItem(DVP_STORAGE_KEYS.nfsShare) || DVP_DEFAULTS.nfsShare;
  var user = sessionStorage.getItem(DVP_STORAGE_KEYS.adminUser) || 'admin';
  var hash = sessionStorage.getItem(DVP_STORAGE_KEYS.adminHash) || sessionStorage.getItem('dhctl-user-password-hash') || '';
  var pubDomain = (sessionStorage.getItem('dhctl-domain') || '').trim() || DVP_DEFAULTS.domain;
  var sshPub = sessionStorage.getItem(DVP_STORAGE_KEYS.ssh) || '';
  var nfsScEff = (sessionStorage.getItem(DVP_STORAGE_KEYS.nfsScName) || '').trim() || DVP_DEFAULTS.nfsStorageClass;
  var dvcrEff = (sessionStorage.getItem(DVP_STORAGE_KEYS.dvcrSize) || '').trim() || DVP_DEFAULTS.dvcrPvcSize;
  var projEff = (sessionStorage.getItem(DVP_STORAGE_KEYS.projectName) || '').trim() || DVP_DEFAULTS.projectName;
  var t = base;
  t = t.split('<POD_SUBNET_CIDR>').join(pod);
  t = t.split('<SERVICE_SUBNET_CIDR>').join(svc);
  t = dvp_split_join_if_nonempty(t, '<INTERNAL_NETWORK_CIDRS>', internal);
  t = dvp_split_join_if_nonempty(t, '<WORKER_NODE_IP>', worker);
  t = t.split('<NFS_HOST>').join(nfsH);
  t = t.split('<NFS_SHARE>').join(nfsS);
  t = dvp_split_join_if_nonempty(t, '<VIRTUAL_MACHINE_CIDRS>', vm);
  t = t.split('<USER_NAME>').join(user);
  t = t.split('<PUBLIC_DOMAIN_TEMPLATE>').join(pubDomain);
  t = dvp_split_join_if_nonempty(t, '<SSH_PUBLIC_KEY>', sshPub);
  t = t.split('<NFS_STORAGE_CLASS_NAME>').join(nfsScEff);
  t = t.split('<DVCR_STORAGE_SIZE>').join(dvcrEff);
  t = t.split('<DVP_PROJECT_NAME>').join(projEff);
  if (hash) {
    t = t.split('<GENERATED_PASSWORD_HASH>').join(hash);
  }
  return t;
}

/** Snippetcut renders syntax highlighting in `.highlight code` and plain text in `[config-yml].snippetcut__raw`; keep both in sync. */
function dvp_sync_highlight_for_config_yml_raw(rawEl) {
  var plain = rawEl.textContent;
  var root = rawEl.closest('[data-snippetcut]');
  if (!root) return;
  var hi = root.querySelector(':scope > .highlight');
  if (!hi) return;
  var codeEl = hi.querySelector('code');
  if (codeEl) {
    codeEl.textContent = plain;
    return;
  }
  hi.textContent = '';
  var pre = document.createElement('pre');
  pre.className = 'highlight';
  var code = document.createElement('code');
  code.className = 'language-yaml';
  code.textContent = plain;
  pre.appendChild(code);
  hi.appendChild(pre);
}

function dvp_sync_all_config_yml_highlights() {
  dvp_get_config_yml_raw_elements().forEach(dvp_sync_highlight_for_config_yml_raw);
}

function dvp_update_cluster_parameters() {
  dvp_get_config_yml_raw_elements().forEach(function (el) {
    var snap = el._dvpYamlSnapshot;
    if (snap === undefined) return;
    el.textContent = dvp_apply_placeholders_to_text(snap);
  });
}

/**
 * Substitute install docs tokens (docker/dhctl) in code spans. Optional session keys:
 * dvp-ssh-private-key-filename, dvp-bootstrap-ssh-user, dvp-master-node-ip (else worker IP).
 */
function dvp_apply_install_command_placeholders() {
  var sshFile = sessionStorage.getItem('dvp-ssh-private-key-filename') || 'id_ed25519';
  var sshUser = sessionStorage.getItem('dvp-bootstrap-ssh-user') || 'ubuntu';
  var masterIp = sessionStorage.getItem('dvp-master-node-ip') || sessionStorage.getItem(DVP_STORAGE_KEYS.worker) || '';

  var installSelectors = '.post-content pre code, .post-content li code, .post-content p code, .post-content td code, .post-content .highlight code, .docs pre code, .docs li code, .docs p code, .docs td code, .docs .highlight code, .layout-sidebar pre code, .layout-sidebar li code, .layout-sidebar p code, .layout-sidebar .highlight code';
  document.querySelectorAll(installSelectors).forEach(function (el) {
    if (el.closest('.snippetcut__raw')) return;
    var t = el.textContent;
    if (t.indexOf('<SSH_PRIVATE_KEY_FILE>') === -1 && t.indexOf('<username>') === -1 && t.indexOf('<master_ip>') === -1) {
      return;
    }
    if (!el.getAttribute(DVP_INSTALL_SNAPSHOT_ATTR)) {
      el.setAttribute(DVP_INSTALL_SNAPSHOT_ATTR, t);
    }
    var base = el.getAttribute(DVP_INSTALL_SNAPSHOT_ATTR);
    var out = base.split('<SSH_PRIVATE_KEY_FILE>').join(sshFile).split('<username>').join(sshUser);
    if (masterIp) {
      out = out.split('<master_ip>').join(masterIp);
    }
    el.textContent = out;
  });
}

/** Видимые блоки (подсветка + dhctl) после любых правок `snippetcut__raw`. */
function dvp_refresh_visible_snippets() {
  dvp_sync_all_config_yml_highlights();
  dvp_apply_install_command_placeholders();
}

function dvp_config_update() {
  dvp_ensure_snippet_snapshots();
  dvp_update_cluster_parameters();
  if (typeof update_domain_parameters === 'function') {
    update_domain_parameters();
  }
  dvp_refresh_visible_snippets();
}

function dvp_toggle_next_step_enabled(valid) {
  var $next = $('#gs-next-step');
  if (!$next.length) return;
  if (valid) {
    $next.removeClass('dvp-next-step--blocked');
    $next.attr('aria-disabled', 'false');
  } else {
    $next.addClass('dvp-next-step--blocked');
    $next.attr('aria-disabled', 'true');
  }
}

function dvp_bind_next_step_guard() {
  $(document).on('click', '#gs-next-step.dvp-next-step--blocked', function (e) {
    e.preventDefault();
    if (typeof dvp_validate_cluster_form === 'function') {
      dvp_validate_cluster_form({ strictCidr: true, skipConfigUpdate: true, showAllErrors: true });
    }
    return false;
  });
}

var dvp_run_validation_debounced = dvp_debounce(function () {
  dvp_validate_cluster_form({ strictCidr: false });
}, 180);

function dvp_bind_form_events() {
  var sel = '#internalnetworkcidrs, #virtualmachinecidrs, #podsubnetcidr, #servicesubnetcidr, #workernodeip, #clusterdomain, #nfsshare, #nfshost, #nfsstoragclassname, #dvcrdisksize, #projectname, #capssshkey, #adminusername';
  $(document).on('input', sel + ', #adminpassword', function () {
    $(this).data('dvpTouched', true);
    dvp_run_validation_debounced();
  });
  $(document).on('blur', sel + ', #adminpassword', function () {
    var $f = $(this);
    if (($f.val() || '').trim().length > 0) {
      $f.data('dvpTouched', true);
    }
    dvp_validate_cluster_form({ strictCidr: true });
  });
}

function dvp_init_cluster_setup_page() {
  if (!$('#internalnetworkcidrs').length) return;

  dvp_restore_data();
  dvp_bind_form_events();
  dvp_bind_next_step_guard();
  dvp_validate_cluster_form({ strictCidr: true });
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', dvp_init_cluster_setup_page);
} else {
  dvp_init_cluster_setup_page();
}

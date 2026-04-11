/**
 * DVP getting started — step «Подготовка»: подстановка в примеры команд значений
 * из sessionStorage (шаг параметров кластера).
 */
(function () {
  var KEYS = {
    nfsHost: 'dvp-nfs-host',
    nfsShare: 'dvp-nfs-share',
    ssh: 'dvp-caps-ssh-public-key',
    internal: 'dvp-internal-network-cidrs',
  };
  var SNAP_ATTR = 'data-dvp-prepare-snapshot';

  function dvpApplyPrepareToCodeElement(codeEl) {
    var raw = codeEl.textContent || '';
    if (
      raw.indexOf('<NFS_HOST>') === -1 &&
      raw.indexOf('<NFS_SHARE>') === -1 &&
      raw.indexOf('<SSH_KEY>') === -1 &&
      raw.indexOf('<SUBNET_CIDR>') === -1
    ) {
      return;
    }
    if (!codeEl.getAttribute(SNAP_ATTR)) {
      codeEl.setAttribute(SNAP_ATTR, raw);
    }
    var base = codeEl.getAttribute(SNAP_ATTR) || raw;
    var nfsH = sessionStorage.getItem(KEYS.nfsHost) || '';
    var nfsS = sessionStorage.getItem(KEYS.nfsShare) || '';
    var ssh = sessionStorage.getItem(KEYS.ssh) || '';
    var subnet = sessionStorage.getItem(KEYS.internal) || '';
    var out = base
      .split('<NFS_HOST>').join(nfsH || '<NFS_HOST>')
      .split('<NFS_SHARE>').join(nfsS || '<NFS_SHARE>')
      .split('<SSH_KEY>').join(ssh || '<SSH_KEY>')
      .split('<SUBNET_CIDR>').join(subnet || '<SUBNET_CIDR>');
    if (out !== codeEl.textContent) {
      codeEl.textContent = out;
    }
  }

  function dvpRunPreparePlaceholders() {
    var roots = document.querySelectorAll(
      '.post-content pre code, .post-content .highlight code, .docs pre code, .layout-sidebar pre code, article pre code'
    );
    roots.forEach(dvpApplyPrepareToCodeElement);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', dvpRunPreparePlaceholders);
  } else {
    dvpRunPreparePlaceholders();
  }
})();

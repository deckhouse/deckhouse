/**
 * DVP getting started — substitutions on prepare / access pages (e.g. step 3).
 * Depends: jQuery, sessionStorage; uses global domain_update from getting-started-access.js when present.
 */

var DVP_PREP_SNAPSHOT_ATTR = 'data-dvp-prepare-snapshot';

function dvp_update_prepare_snippets() {
  if (typeof $ === 'undefined') return;

  var key = sessionStorage.getItem('dvp-caps-ssh-public-key') || '';
  var share = sessionStorage.getItem('dvp-nfs-share') || '/srv/nfs/dvp';
  var host = sessionStorage.getItem('dvp-nfs-host') || '192.168.1.100';
  var sub = sessionStorage.getItem('dvp-internal-network-cidrs') || '';

  $('pre code, .highlight code').each(function () {
    var el = this;
    var t = el.textContent;
    if (t.indexOf('<SSH_KEY>') === -1 && t.indexOf('<NFS_SHARE>') === -1 && t.indexOf('<NFS_HOST>') === -1 && t.indexOf('<SUBNET_CIDR>') === -1) {
      return;
    }
    if (!el.getAttribute(DVP_PREP_SNAPSHOT_ATTR)) {
      el.setAttribute(DVP_PREP_SNAPSHOT_ATTR, t);
    }
    var base = el.getAttribute(DVP_PREP_SNAPSHOT_ATTR);
    el.textContent = base.split('<SSH_KEY>').join(key).split('<NFS_SHARE>').join(share).split('<NFS_HOST>').join(host).split('<SUBNET_CIDR>').join(sub);
  });
}

function dvp_domain_update() {
  if (typeof domain_update === 'function') {
    domain_update();
  }
}

document.addEventListener('DOMContentLoaded', function () {
  dvp_update_prepare_snippets();
  dvp_domain_update();
});

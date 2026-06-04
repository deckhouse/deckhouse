// DVP GS step 2: cluster setup form → sessionStorage (DVP-only; DKP uses step_cluster_setup.html).

// Debounced input handler.
function debounce(fn, ms) {
  let t;
  return function () {
    const ctx = this;
    const args = arguments;
    clearTimeout(t);
    t = setTimeout(function () {
      fn.apply(ctx, args);
    }, ms);
  };
}

// Parse IPv4 to uint32.
function parseIpv4(s) {
  const m = String(s).trim().match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
  if (!m) return null;
  const p = [1, 2, 3, 4].map(function (i) {
    return parseInt(m[i], 10);
  });
  if (p.some(function (x) {
    return x > 255 || x < 0;
  })) return null;
  return (p[0] * 16777216 + p[1] * 65536 + p[2] * 256 + p[3]) >>> 0;
}

// Parse CIDR → { start, end }.
function parseIpv4Cidr(str) {
  const s = String(str).trim();
  const m = s.match(/^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\/(\d{1,2})$/);
  if (!m) return null;
  const pref = parseInt(m[2], 10);
  if (pref < 0 || pref > 32) return null;
  const ip = parseIpv4(m[1]);
  if (ip === null) return null;
  const mask = pref === 0 ? 0 : (~((Math.pow(2, 32 - pref) - 1) >>> 0)) >>> 0;
  const network = (ip & mask) >>> 0;
  const size = pref === 32 ? 1 : Math.pow(2, 32 - pref);
  const end = (network + size - 1) >>> 0;
  return { start: network, end: end, raw: s };
}

// CIDR overlap check for subnet fields.
function cidrsOverlap(a, b) {
  const A = parseIpv4Cidr(a);
  const B = parseIpv4Cidr(b);
  if (!A || !B) return false;
  return !(A.end < B.start || B.end < A.start);
}

// Looks like x.x.x.x/nn (may still be invalid).
function cidrLooksComplete(s) {
  return /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\/\d{1,2}$/.test(String(s).trim());
}

// Scope error messages to one .form__field.
function fieldWrap($input) {
  const $field = $input.closest('.form__field');
  if ($field.length) return $field;
  return $input.parent();
}

// Restore one input from sessionStorage.
function updateNode(selector, storageKey) {
  const v = sessionStorage.getItem(storageKey);
  if (v && v.length > 0) {
    $(selector).val(v);
  }
}

// Step 2: refill form when user navigates back.
function restoreData() {
  if (!$('#internalnetworkcidrs').length) return;

  updateNode('#internalnetworkcidrs', STORAGE_KEYS.internal);
  updateNode('#virtualmachinecidrs', STORAGE_KEYS.vm);
  updateNode('#podsubnetcidr', STORAGE_KEYS.pod);
  updateNode('#servicesubnetcidr', STORAGE_KEYS.service);
  updateNode('#workernodeip', STORAGE_KEYS.worker);
  updateNode('#clusterdomain', STORAGE_KEYS.domain);
  updateNode('#nfsshare', STORAGE_KEYS.nfsShare);
  updateNode('#nfshost', STORAGE_KEYS.nfsHost);
  updateNode('#masternodeip', STORAGE_KEYS.master);
}

// Clear validation UI for one field.
function clearFieldErrors($input) {
  const $wrap = fieldWrap($input);
  $input.removeClass('invalid');
  $input.removeAttr('aria-invalid');
  $wrap.find('.invalid-message-main').removeClass('active');
  $wrap.find('.invalid-message-example-com').removeClass('active');
  $wrap.find('.invalid-message-overlap').removeClass('active');
  $wrap.find('.invalid-message-duplicate').removeClass('active');
  $wrap.find('.invalid-message-nfs-subnet').removeClass('active');
  $wrap.find('.invalid-message-nfs-node').removeClass('active');
}

// Show validation message (main/overlap/duplicate/example).
function setFieldError($input, kind) {
  const $wrap = fieldWrap($input);
  $input.addClass('invalid');
  $input.attr('aria-invalid', 'true');
  if (kind === 'example') {
    $wrap.find('.invalid-message-example-com').addClass('active');
  } else if (kind === 'overlap') {
    $wrap.find('.invalid-message-overlap').addClass('active');
  } else if (kind === 'duplicate') {
    $wrap.find('.invalid-message-duplicate').addClass('active');
  } else if (kind === 'nfs-subnet') {
    $wrap.find('.invalid-message-nfs-subnet').addClass('active');
  } else if (kind === 'nfs-node') {
    $wrap.find('.invalid-message-nfs-node').addClass('active');
  } else {
    $wrap.find('.invalid-message-main').addClass('active');
  }
}

// Show error only if field touched or forced (Next click).
function maybeSetFieldError($input, kind, options) {
  if (!$input || !$input.length) return;
  if (kind === 'overlap' || kind === 'duplicate' || kind === 'nfs-subnet' || kind === 'nfs-node') {
    setFieldError($input, kind);
    return;
  }
  if (options && options.showAllErrors) {
    setFieldError($input, kind);
    return;
  }
  if ($input.data('gsTouched')) {
    setFieldError($input, kind);
  }
}

// Step 2: validate form, persist to sessionStorage, enable Next.
function validateClusterForm(options) {
  options = options || {};
  const strictCidr = options.strictCidr === true;

  // Wrapper: respect forceShowErrors from Next click.
  function maybeErr($el, k) {
    maybeSetFieldError($el, k, options);
  }

  const ids = [
    '#internalnetworkcidrs', '#virtualmachinecidrs', '#podsubnetcidr', '#servicesubnetcidr',
    '#masternodeip', '#workernodeip', '#clusterdomain', '#nfsshare', '#nfshost',
  ];
  ids.forEach(function (sel) {
    const $el = $(sel);
    if ($el.length) clearFieldErrors($el);
  });
  $('#gs-overlap-banner').removeClass('active');

  let ok = true;
  const internal = ($('#internalnetworkcidrs').val() || '').trim();
  const vm = ($('#virtualmachinecidrs').val() || '').trim();
  const podInput = ($('#podsubnetcidr').val() || '').trim();
  const svcInput = ($('#servicesubnetcidr').val() || '').trim();
  const workerIp = ($('#workernodeip').val() || '').trim();
  const domain = ($('#clusterdomain').val() || '').trim();
  const nfsShareInput = ($('#nfsshare').val() || '').trim();
  const nfsPath = nfsShareInput || DEFAULTS.nfsShare;
  const nfsHostInput = ($('#nfshost').val() || '').trim();
  const masterIp = ($('#masternodeip').val() || '').trim();

  // Validate one CIDR field (required/strict modes).
  function validateCidrField($field, val, required) {
    if (!val) {
      if (required) {
        maybeErr($field, 'main');
        ok = false;
      }
      return false;
    }
    if (!cidrLooksComplete(val)) {
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
    if (!parseIpv4Cidr(val)) {
      maybeErr($field, 'main');
      ok = false;
      return false;
    }
    return true;
  }

  const internalValid = validateCidrField($('#internalnetworkcidrs'), internal, true);

  const vmEffective = vm || DEFAULTS.vm;
  let vmValid = true;
  if (vm) {
    vmValid = validateCidrField($('#virtualmachinecidrs'), vm, false);
  } else if (!parseIpv4Cidr(vmEffective)) {
    ok = false;
    vmValid = false;
  }

  const podEffective = podInput || DEFAULTS.pod;
  let podValid = true;
  if (podInput) {
    podValid = validateCidrField($('#podsubnetcidr'), podInput, false);
  } else if (!parseIpv4Cidr(podEffective)) {
    ok = false;
    podValid = false;
  }

  const svcEffective = svcInput || DEFAULTS.service;
  let svcValid = true;
  if (svcInput) {
    svcValid = validateCidrField($('#servicesubnetcidr'), svcInput, false);
  } else if (!parseIpv4Cidr(svcEffective)) {
    ok = false;
    svcValid = false;
  }

  if ($('#masternodeip').length) {
    const $mEl = $('#masternodeip');
    if (!masterIp) {
      maybeErr($mEl, 'main');
      ok = false;
    } else if (parseIpv4(masterIp) === null) {
      maybeErr($mEl, 'main');
      ok = false;
    }
  }

  if ($('#workernodeip').length) {
    const $wEl = $('#workernodeip');
    if (!workerIp) {
      maybeErr($wEl, 'main');
      ok = false;
    } else if (parseIpv4(workerIp) === null) {
      maybeErr($wEl, 'main');
      ok = false;
    } else if (internalValid && parseIpv4Cidr(internal) && !ipv4InCidr(workerIp, internal)) {
      maybeErr($wEl, 'main');
      ok = false;
    }
  }

  if (masterIp && workerIp && masterIp === workerIp) {
    maybeErr($('#masternodeip'), 'duplicate');
    maybeErr($('#workernodeip'), 'duplicate');
    ok = false;
  }

  if (domain) {
    if (!domain.match(PUBLIC_DOMAIN_PATTERN)) {
      maybeErr($('#clusterdomain'), 'main');
      ok = false;
    } else if (domain.match(/\.example\.com/)) {
      maybeErr($('#clusterdomain'), 'example');
      ok = false;
    }
  }

  if (nfsShareInput.length > 0 && !/^\/[A-Za-z0-9_\-\/]*$/.test(nfsPath)) {
    maybeErr($('#nfsshare'), 'main');
    ok = false;
  }

  if (!nfsHostInput) {
    maybeErr($('#nfshost'), 'main');
    ok = false;
  } else if (parseIpv4(nfsHostInput) === null) {
    maybeErr($('#nfshost'), 'main');
    ok = false;
  } else {
    if (internalValid && parseIpv4Cidr(internal) && !ipv4InCidr(nfsHostInput, internal)) {
      maybeErr($('#nfshost'), 'nfs-subnet');
      ok = false;
    }
    if (masterIp && nfsHostInput === masterIp) {
      maybeErr($('#nfshost'), 'nfs-node');
      ok = false;
    }
    if (workerIp && nfsHostInput === workerIp) {
      maybeErr($('#nfshost'), 'nfs-node');
      ok = false;
    }
  }

  const cidrList = [];
  if (internalValid) cidrList.push({ id: '#internalnetworkcidrs', c: internal });
  if (vmValid) cidrList.push({ id: '#virtualmachinecidrs', c: vmEffective });
  if (podValid) cidrList.push({ id: '#podsubnetcidr', c: podEffective });
  if (svcValid) cidrList.push({ id: '#servicesubnetcidr', c: svcEffective });

  if (cidrList.length >= 2) {
    for (let i = 0; i < cidrList.length; i++) {
      for (let j = i + 1; j < cidrList.length; j++) {
        if (cidrsOverlap(cidrList[i].c, cidrList[j].c)) {
          maybeSetFieldError($(cidrList[i].id), 'overlap', options);
          maybeSetFieldError($(cidrList[j].id), 'overlap', options);
          $('#gs-overlap-banner').addClass('active');
          ok = false;
        }
      }
    }
  }

  if (ok) {
    sessionStorage.setItem(STORAGE_KEYS.internal, internal);
    if (vm) {
      sessionStorage.setItem(STORAGE_KEYS.vm, vm);
    } else {
      sessionStorage.removeItem(STORAGE_KEYS.vm);
    }
    if (podInput) {
      sessionStorage.setItem(STORAGE_KEYS.pod, podInput);
    } else {
      sessionStorage.removeItem(STORAGE_KEYS.pod);
    }
    if (svcInput) {
      sessionStorage.setItem(STORAGE_KEYS.service, svcInput);
    } else {
      sessionStorage.removeItem(STORAGE_KEYS.service);
    }
    if ($('#workernodeip').length) {
      sessionStorage.setItem(STORAGE_KEYS.worker, workerIp);
    }
    if ($('#masternodeip').length) {
      sessionStorage.setItem(STORAGE_KEYS.master, masterIp);
    }
    if (domain) {
      sessionStorage.setItem(STORAGE_KEYS.domain, domain);
    } else {
      sessionStorage.removeItem(STORAGE_KEYS.domain);
    }
    sessionStorage.setItem(STORAGE_KEYS.nfsShare, nfsShareInput || DEFAULTS.nfsShare);
    sessionStorage.setItem(STORAGE_KEYS.nfsHost, nfsHostInput);
  }

  toggleNextStepEnabled(ok);

  return ok;
}

// IP inside CIDR range.
function ipv4InCidr(ip, cidr) {
  const n = parseIpv4(ip);
  const c = parseIpv4Cidr(cidr);
  if (n === null || !c) return false;
  return n >= c.start && n <= c.end;
}

// #gs-next-step blocked until form valid.
function toggleNextStepEnabled(valid) {
  const $next = $('#gs-next-step');
  if (!$next.length) return;
  if (valid) {
    $next.removeClass('gs-next-step--blocked');
    $next.attr('aria-disabled', 'false');
    const saved = $next.attr('data-href');
    if (saved) {
      $next.attr('href', saved);
    }
    $next.css('pointer-events', '');
  } else {
    const href = $next.attr('href');
    if (href && href !== '#') {
      $next.attr('data-href', href);
    }
    $next.attr('href', '#');
    $next.css('pointer-events', 'none');
    $next.addClass('gs-next-step--blocked');
    $next.attr('aria-disabled', 'true');
  }
}

// Blocked Next: show all errors on click.
function bindNextStepGuard() {
  $(document).on('click', '#gs-next-step.gs-next-step--blocked', function (e) {
    e.preventDefault();
    if (typeof validateClusterForm === 'function') {
      validateClusterForm({ strictCidr: true, showAllErrors: true });
    }
    return false;
  });
}

const runValidationDebounced = debounce(function () {
  validateClusterForm({ strictCidr: false });
}, 180);

// input/blur listeners on cluster form.
function bindFormEvents() {
  const sel = '#internalnetworkcidrs, #virtualmachinecidrs, #podsubnetcidr, #servicesubnetcidr, #masternodeip, #workernodeip, #clusterdomain, #nfsshare, #nfshost';
  $(document).on('input', sel, function () {
    $(this).data('gsTouched', true);
    runValidationDebounced();
  });
  $(document).on('blur', sel, function () {
    const $f = $(this);
    if (($f.val() || '').trim().length > 0) {
      $f.data('gsTouched', true);
    }
    validateClusterForm({ strictCidr: true });
  });
}

// Step 2 entry: restore, bind, initial validate.
function initClusterSetupPage() {
  if (!$('#internalnetworkcidrs').length) return;

  restoreData();
  bindFormEvents();
  bindNextStepGuard();
  validateClusterForm({ strictCidr: true });
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initClusterSetupPage);
} else {
  initClusterSetupPage();
}

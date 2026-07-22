// DVP GS: <TOKEN> and *.domain.my substitution in snippets (requires getting-started-dvp-constants.js).
// DKP uses [config-yml] + update_parameter; DVP uses this layer for .dvp-config-yaml and command blocks.

function dvpEffectivePublicDomainTemplate() {
  return dvpSessionValues().pubDomain;
}

// Docs are authored with *.domain.my; match that suffix regardless of user template.
function dvpAuthoredDnsHostnameRe() {
  var suffix = DEFAULTS.domainSuffix.replace(/\./g, '\\.');
  return new RegExp('([a-z0-9-]+)\\.' + suffix, 'g');
}

var GS_SNAPSHOT_ATTR = 'data-gs-snapshot';

// Skip line numbers / copy-ignore nodes when walking DOM.
function gsSkipCopyIgnoreNode(node) {
  if (!node || node.nodeType !== Node.ELEMENT_NODE) {
    return false;
  }
  return node.getAttribute('data-copy') === 'ignore' || node.classList.contains('line-number');
}

function dvpTextHasDnsPlaceholders(text) {
  if (!text) {
    return false;
  }
  var re = dvpAuthoredDnsHostnameRe();
  re.lastIndex = 0;
  return re.test(text);
}

function gsApplyDnsHostnameToString(str, template) {
  if (!str) {
    return str;
  }
  if (template && template !== DEFAULTS.domain) {
    str = str.split(DEFAULTS.domain).join(template);
  }
  var authoredRe = dvpAuthoredDnsHostnameRe();
  authoredRe.lastIndex = 0;
  return str.replace(authoredRe, function (_, service) {
    return template.replace('%s', service);
  });
}

// Step 6: console link from domain template.
function gsReplaceDnsHostnamesInLinks(root) {
  var template = dvpEffectivePublicDomainTemplate();
  var scope = root || document;

  scope.querySelectorAll('a[href]').forEach(function (a) {
    if (gsSkipCopyIgnoreNode(a)) {
      return;
    }
    var href = a.getAttribute('href');
    var newHref = gsApplyDnsHostnameToString(href, template);
    if (newHref !== href) {
      a.setAttribute('href', newHref);
    }
    var text = a.textContent;
    var newText = gsApplyDnsHostnameToString(text, template);
    if (newText !== text) {
      a.textContent = newText;
    }
  });
}

// *.<suffix> hostnames inside a code element.
function gsReplaceDnsHostnamesInTextNodes(el) {
  var template = dvpEffectivePublicDomainTemplate();
  var walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, null);
  var node;

  while ((node = walker.nextNode())) {
    if (gsSkipCopyIgnoreNode(node.parentElement)) {
      continue;
    }
    var data = node.data;
    var newData = gsApplyDnsHostnameToString(data, template);
    if (newData !== data) {
      node.data = newData;
    }
  }
}

function gsApplyReplacementsToString(str, replacements) {
  if (!str) {
    return str;
  }
  var newText = str;
  replacements.forEach(function (r) {
    if (newText.indexOf(r.token) === -1) {
      return;
    }
    var val = r.getValue();
    newText = newText.split(r.token).join(val === '' && r.keepTokenIfEmpty ? r.token : val);
  });
  return newText;
}

// Replace <TOKEN> placeholders in a code block (text nodes, then whole block if needed).
function gsReplaceTokensInTextNodes(el, replacements) {
  var walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, null);
  var node;

  while ((node = walker.nextNode())) {
    if (gsSkipCopyIgnoreNode(node.parentElement)) {
      continue;
    }
    var data = node.data;
    var newData = gsApplyReplacementsToString(data, replacements);
    if (newData !== data) {
      node.data = newData;
    }
  }

  // Syntax-highlight markup may split <TOKEN> across spans; normalize via textContent when tokens remain.
  var text = el.textContent || '';
  var newText = gsApplyReplacementsToString(text, replacements);
  if (newText !== text) {
    el.textContent = newText;
  }
}

function dvpFilterPlaceholderRules(rules) {
  return rules.filter(function (rule) {
    if (!rule.omitWhenFalsy) {
      return true;
    }
    var val = rule.getValue();
    return val !== null && val !== undefined && val !== '';
  });
}

// Placeholder rules for command snippets or config.yml (.dvp-config-yaml).
function dvpPlaceholderRules(values, scope) {
  var v = values || dvpSessionValues();
  var rules = [];

  function add(rule) {
    rules.push(rule);
  }

  if (scope === 'config') {
    // Longest token names first so substring replacement stays safe.
    add({
      token: '<GENERATED_PASSWORD_HASH>',
      getValue: function () { return v.hash; },
      keepTokenIfEmpty: true,
      omitWhenFalsy: true,
    });
    add({
      token: '<CAPS_SSH_PRIVATE_KEY_BASE64>',
      getValue: function () { return v.capsPrivateKeyB64; },
      keepTokenIfEmpty: true,
      omitWhenFalsy: true,
    });
    add({
      token: '<YOUR_ACCESS_STRING_IS_HERE>',
      getValue: function () { return v.licenseCfg; },
      keepTokenIfEmpty: true,
      omitWhenFalsy: true,
    });
    add({
      token: '<GENERATED_PASSWORD>',
      getValue: function () { return v.password; },
      keepTokenIfEmpty: true,
      omitWhenFalsy: true,
    });
    add({
      token: '<WORKER_NODE_IP>',
      getValue: function () { return v.workerIp; },
      keepTokenIfEmpty: true,
      omitWhenFalsy: true,
    });
  }

  if (scope === 'command') {
    add({
      token: '<CAPS_SSH_PRIVATE_KEY>',
      getValue: dvpCapsPrivateKeyPem,
      keepTokenIfEmpty: true,
    });
    add({
      token: '<MASTER_IP>',
      getValue: function () { return v.master; },
      keepTokenIfEmpty: true,
    });
    add({
      token: '<GENERATED_PASSWORD>',
      getValue: function () { return v.password; },
      keepTokenIfEmpty: true,
    });
  }

  add({ token: '<NFS_SHARE>', getValue: function () { return v.nfsS; }, keepTokenIfEmpty: scope === 'command' });
  add({ token: '<NFS_HOST>', getValue: function () { return v.nfsH; }, keepTokenIfEmpty: true });
  add({ token: '<INTERNAL_NETWORK_CIDRS>', getValue: function () { return v.internal; }, keepTokenIfEmpty: true });
  add({ token: '<POD_SUBNET_CIDR>', getValue: function () { return v.pod; } });
  add({ token: '<SERVICE_SUBNET_CIDR>', getValue: function () { return v.svc; } });
  add({ token: '<VIRTUAL_MACHINE_CIDRS>', getValue: function () { return v.vm; } });
  add({ token: '<PUBLIC_DOMAIN_TEMPLATE>', getValue: function () { return v.pubDomain; } });
  add({
    token: '<CAPS_SSH_PUBLIC_KEY>',
    getValue: function () { return v.capsPublicKey; },
    keepTokenIfEmpty: true,
  });

  if (scope === 'command') {
    add({
      token: '<WORKER_NODE_IP>',
      getValue: function () { return v.workerIp; },
      keepTokenIfEmpty: true,
    });
  }

  return dvpFilterPlaceholderRules(rules);
}

function dvpCodeNeedsPlaceholders(text, rules) {
  if (!text) {
    return false;
  }
  if (dvpTextHasDnsPlaceholders(text)) {
    return true;
  }
  return rules.some(function (r) {
    return text.indexOf(r.token) !== -1;
  });
}

var DVP_COMMAND_SNIPPET_SELECTORS =
  '.post-content pre code, .post-content li code, .post-content p code, .post-content td code, ' +
  '.post-content .highlight code, .docs pre code, .docs li code, .docs p code, .docs td code, ' +
  '.docs .highlight code, .layout-sidebar pre code, .layout-sidebar li code, .layout-sidebar p code, ' +
  '.layout-sidebar .highlight code, article pre code, article .highlight code';

function gsEnsureSnapshotBaseline(codeEl, snapshotAttr) {
  var htmlAttr = snapshotAttr + '-html';
  if (!codeEl.getAttribute(htmlAttr)) {
    codeEl.setAttribute(htmlAttr, codeEl.innerHTML);
  }
}

function gsRestoreSnapshotBaseline(codeEl, snapshotAttr) {
  gsEnsureSnapshotBaseline(codeEl, snapshotAttr);
  codeEl.innerHTML = codeEl.getAttribute(snapshotAttr + '-html');
}

function dvpApplyTokensToElement(codeEl, rules, snapshotAttr) {
  if (!codeEl || !rules.length) {
    return;
  }
  gsRestoreSnapshotBaseline(codeEl, snapshotAttr);
  gsReplaceTokensInTextNodes(codeEl, rules);
}

// Patch one command <code> (tokens + DNS).
function dvpApplyCommandPlaceholdersToElement(codeEl, snapshotAttr) {
  if (!codeEl) {
    return;
  }
  var rules = dvpPlaceholderRules(dvpSessionValues(), 'command');
  var t = codeEl.textContent || '';
  if (!dvpCodeNeedsPlaceholders(t, rules)) {
    return;
  }

  dvpApplyTokensToElement(codeEl, rules, snapshotAttr);
  gsReplaceDnsHostnamesInTextNodes(codeEl);
}

// All command snippets on the page (config.yml uses install.js config pass).
function dvpApplyCommandPlaceholders(root) {
  var scope = root || document;
  scope.querySelectorAll(DVP_COMMAND_SNIPPET_SELECTORS).forEach(function (el) {
    if (el.closest('.dvp-config-yaml')) {
      return;
    }
    dvpApplyCommandPlaceholdersToElement(el, GS_SNAPSHOT_ATTR);
  });
}

function dvp_refresh_command_placeholders() {
  dvpApplyCommandPlaceholders(document);
}

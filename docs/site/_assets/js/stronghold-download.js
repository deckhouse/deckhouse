// Download block of the Stronghold "Linux OS" getting started guide.
//
// For every [data-stronghold-download] block (one per edition tab):
//  - prefill the license key from the `license-token` cookie shared with the
//    DKP getting started guide;
//  - on "Show versions" request the version list from the download service
//    (registry-proxy) with the key in the X-License-Token header;
//  - on "Download" submit a hidden POST form so the key never appears in a URL.
//
// The key is not validated on the site; the service answers 401 for a bad key.
// Configuration comes from the `strongholdDownload` global defined in
// _includes/getting_started/stronghold/linux/partials/download.html.liquid.

(function () {
  const LICENSE_COOKIE = 'license-token';
  const VERSION_PATTERN = /stronghold-v[\w.+-]+\.tar/g;

  function config() {
    return window.strongholdDownload || { prefix: '/products/stronghold/get', i18n: {} };
  }

  function readCookie() {
    if (window.$ && $.cookie) {
      return $.cookie(LICENSE_COOKIE) || $.cookie('demotoken') || '';
    }
    return '';
  }

  function saveCookie(token) {
    if (window.$ && $.cookie) {
      $.cookie(LICENSE_COOKIE, token, { path: '/', expires: 1 });
    }
  }

  function setMessage(el, text, isError) {
    el.innerHTML = text || '';
    el.className = text ? (isError ? 'license-form__warn' : 'license-form__message') : '';
  }

  function setInputState(input, state) {
    input.classList.remove('license-token-input--error', 'license-token-input--success');
    if (state) {
      input.classList.add('license-token-input--' + state);
    }
  }

  // Best-effort: keep the unpack snippet in sync with the chosen version.
  function updateUnpackSnippet(version) {
    const snippet = document.getElementById('stronghold-unpack');
    if (!snippet || !version) return;
    snippet.querySelectorAll('code, [data-snippetcut-text]').forEach(function (el) {
      const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
      let node;
      while ((node = walker.nextNode())) {
        if (VERSION_PATTERN.test(node.nodeValue)) {
          node.nodeValue = node.nodeValue.replace(VERSION_PATTERN, 'stronghold-' + version + '.tar');
        }
        VERSION_PATTERN.lastIndex = 0;
      }
    });
  }

  function initBlock(block) {
    const cfg = config();
    const edition = block.getAttribute('data-edition') || 'ee';
    const input = block.querySelector('[data-license-input]');
    const button = block.querySelector('[data-versions-btn]');
    const message = block.querySelector('[data-message]');
    const versionBlock = block.querySelector('[data-version-block]');
    const select = block.querySelector('[data-version-select]');
    const form = block.querySelector('[data-download-form]');

    if (!input || !button || !message || !versionBlock || !select || !form) return;

    const licenseField = form.querySelector('input[name="license"]');

    const saved = readCookie();
    if (saved) {
      input.value = saved;
    }

    function hideVersions() {
      versionBlock.style.display = 'none';
      select.innerHTML = '';
    }

    function showVersions(versions) {
      select.innerHTML = '';
      versions.forEach(function (v) {
        const option = document.createElement('option');
        option.value = v;
        option.textContent = v;
        select.appendChild(option);
      });
      versionBlock.style.display = '';
      updateUnpackSnippet(select.value);
    }

    async function fetchVersions() {
      const token = input.value.trim();
      if (token === '') {
        setMessage(message, cfg.i18n.empty_input, true);
        setInputState(input, 'error');
        hideVersions();
        return;
      }

      button.classList.add('button_disabled');
      try {
        const response = await fetch(cfg.prefix + '/api/versions?edition=' + encodeURIComponent(edition), {
          headers: { 'X-License-Token': token },
          cache: 'no-store'
        });

        if (response.status === 401) {
          setMessage(message, cfg.i18n.reject, true);
          setInputState(input, 'error');
          hideVersions();
          return;
        }
        if (!response.ok) {
          throw new Error('HTTP ' + response.status);
        }

        const data = await response.json();
        const versions = Array.isArray(data.versions) ? data.versions : [];

        saveCookie(token);
        setInputState(input, 'success');

        if (versions.length === 0) {
          setMessage(message, cfg.i18n.no_versions, true);
          hideVersions();
          return;
        }

        setMessage(message, cfg.i18n.resolve, false);
        showVersions(versions);
      } catch (e) {
        setMessage(message, cfg.i18n.network, true);
        setInputState(input, 'error');
        hideVersions();
      } finally {
        button.classList.remove('button_disabled');
      }
    }

    button.addEventListener('click', function (e) {
      e.preventDefault();
      fetchVersions();
    });

    input.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        fetchVersions();
      }
    });

    input.addEventListener('input', function () {
      setInputState(input, null);
      setMessage(message, '', false);
      hideVersions();
    });

    select.addEventListener('change', function () {
      updateUnpackSnippet(select.value);
    });

    form.addEventListener('submit', function (e) {
      const version = select.value;
      const token = input.value.trim();
      if (!version || !token) {
        e.preventDefault();
        return;
      }
      form.action = cfg.prefix + '/download/' + encodeURIComponent(version);
      licenseField.value = token;
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('[data-stronghold-download]').forEach(initBlock);
  });
})();

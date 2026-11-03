document.addEventListener('DOMContentLoaded', function () {
  // Pages without a twin in the other language (CSE documentation, see
  // docs/documentation/_layouts/default.html) get the switch from the shared
  // header/footer through SSI anyway; drop it instead of linking to a 404.
  if (document.querySelector('meta[name="page:lang-switch"][content="disabled"]')) {
    $('a.lang-switcher').remove();
    return;
  }

  // Russian pages have no #language-switch, only the footer link - the
  // a.lang-switcher rewrite at the bottom must still run for them.
  const mobileLanguageSwitch = document.querySelector('#language-switch');

  const isRuPath = (pathname) => /^\/ru(\/|$)/.test(pathname);
  const isEnPath = (pathname) => /^\/en(\/|$)/.test(pathname);

  function swapHostname(hostname) {
    const staticMap = {
      'deckhouse.io': 'deckhouse.ru',
      'deckhouse.ru': 'deckhouse.io',
      'localhost': 'ru.localhost',
      'ru.localhost': 'localhost'
    };

    if (staticMap[hostname]) return staticMap[hostname];
    if (hostname.includes('deckhouse.ru.')) return hostname.replace('deckhouse.ru.', 'deckhouse.');
    if (hostname.includes('deckhouse-ru')) return hostname.replace('deckhouse-ru', 'deckhouse');
    if (hostname.includes('deckhouse')) return hostname.replace('deckhouse', 'deckhouse-ru');

    return null;
  }

  function buildTargetUrl() {
    const url = new URL(window.location.href);

    if (isRuPath(url.pathname)) {
      url.pathname = url.pathname.replace(/^\/ru(?=\/|$)/, '/en');
      return url.toString();
    }

    if (isEnPath(url.pathname)) {
      url.pathname = url.pathname.replace(/^\/en(?=\/|$)/, '/ru');
      return url.toString();
    }

    const newHostname = swapHostname(url.hostname);
    if (!newHostname) return null;

    url.hostname = newHostname;
    return url.toString();
  }

  if (mobileLanguageSwitch) {
    function mobileSyncCheckedState() {
      const { pathname, hostname } = new URL(window.location.href);
      const isRuHost =
        hostname === 'deckhouse.ru' ||
        hostname === 'ru.localhost' ||
        hostname.includes('deckhouse-ru');

      mobileLanguageSwitch.checked = isRuPath(pathname) || isRuHost;
    }

    let mobileIsNavigating = false;
    mobileLanguageSwitch.removeAttribute('onclick');
    mobileLanguageSwitch.addEventListener('change', function () {
      if (window.innerWidth >= 1024 || mobileIsNavigating) return;

      const targetUrl = buildTargetUrl();
      if (!targetUrl || targetUrl === window.location.href) return;

      mobileIsNavigating = true;
      window.location.assign(targetUrl);
    });

    mobileSyncCheckedState();
  }

  const targetUrl = buildTargetUrl();
  if (targetUrl && targetUrl !== window.location.href) {
    $('a.lang-switcher').each(function () {
        $(this).attr('href', targetUrl);
    });
  }

});

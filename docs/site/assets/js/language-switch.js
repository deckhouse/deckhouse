document.addEventListener('DOMContentLoaded', function () {
  const mobileLanguageSwitch = document.querySelector('#language-switch');
  if (!mobileLanguageSwitch) return;

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

  const targetUrl = buildTargetUrl();
  if (targetUrl && targetUrl !== window.location.href) {
    $('a.lang-switcher').each(function () {
        $(this).attr('href', targetUrl);
    });
  }

});

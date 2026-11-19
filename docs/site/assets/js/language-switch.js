// Points the footer's language link at the same page in the other language.
//
// The footer is also served over SSI on pages it knows nothing about, so the
// link carries a fallback address, and the page it is on is only known here.
// Paths with a language prefix (the in-cluster build) swap the prefix; the
// public site swaps the host.
document.addEventListener('DOMContentLoaded', function () {
  // Pages without a twin in the other language (CSE documentation, see
  // docs/documentation/_layouts/default.html) get the footer through SSI
  // anyway; drop its language link instead of pointing it at a 404.
  if (document.querySelector('meta[name="page:lang-switch"][content="disabled"]')) {
    document.querySelectorAll('a.lang-switcher').forEach(function (link) {
      link.remove();
    });
    return;
  }

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

  const targetUrl = buildTargetUrl();
  if (!targetUrl || targetUrl === window.location.href) return;

  document.querySelectorAll('a.lang-switcher').forEach(function (link) {
    link.setAttribute('href', targetUrl);
  });
});

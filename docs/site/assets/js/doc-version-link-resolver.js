(function () {
  const MENU_LINK_SELECTOR = '#doc-versions-menu a.submenu-item-link, #doc-versions-menu a.submenu-embedded-link';
  const CHANNEL_NAMES = new Set(['alpha', 'beta', 'early-access', 'stable', 'rock-solid', 'latest']);
  const PRODUCT_VERSION_RE = /^v\d+\.\d+$/;
  const CACHE_KEY = 'doc-version-link-resolver-cache';
  const DEBUG_QUERY_PARAM = 'debug';
  const DEBUG_STORAGE_KEY = 'doc-version-link-resolver-debug';
  const HISTORICAL_VERSION_THRESHOLD = { major: 1, minor: 72 };

  const debugEnabled = isDebugEnabled();
  const availabilityCache = loadAvailabilityCache();

  function isDebugEnabled() {
    try {
      const searchParams = new URLSearchParams(window.location.search);

      if (searchParams.has(DEBUG_QUERY_PARAM)) {
        const queryValue = searchParams.get(DEBUG_QUERY_PARAM);
        const enabled = queryValue !== '0' && queryValue !== 'false';
        window.sessionStorage.setItem(DEBUG_STORAGE_KEY, enabled ? 'true' : 'false');
        return enabled;
      }

      return window.sessionStorage.getItem(DEBUG_STORAGE_KEY) === 'true';
    } catch (error) {
      return false;
    }
  }

  function debugLog(message, details) {
    if (!debugEnabled) {
      return;
    }

    if (typeof details === 'undefined') {
      console.debug('[doc-version-link-resolver]', message);
      return;
    }

    console.debug('[doc-version-link-resolver]', message, details);
  }

  function loadAvailabilityCache() {
    debugLog('Loading availability cache.');

    try {
      return JSON.parse(window.sessionStorage.getItem(CACHE_KEY) || '{}');
    } catch (error) {
      debugLog('Failed to parse availability cache, using empty cache.', error);
      return {};
    }
  }

  function saveAvailabilityCache() {
    try {
      window.sessionStorage.setItem(CACHE_KEY, JSON.stringify(availabilityCache));
    } catch (error) {
      // Ignore storage failures and continue with in-memory cache.
      debugLog('Failed to persist availability cache.', error);
    }
  }

  function normalizeTail(value) {
    const normalized = (value || '')
      .replace(/^\/+/, '')
      .replace(/\/+$/, '');

    if (normalized === 'readme.html' || normalized === 'index.html') {
      return '';
    }

    return normalized;
  }

  function parseProductVersion(version) {
    const match = (version || '').match(/^v(\d+)\.(\d+)$/);

    if (!match) {
      return null;
    }

    return {
      major: Number(match[1]),
      minor: Number(match[2]),
    };
  }

  function isHistoricalVersionBeforeThreshold(version) {
    const parsedVersion = parseProductVersion(version);

    if (!parsedVersion) {
      return false;
    }

    if (parsedVersion.major !== HISTORICAL_VERSION_THRESHOLD.major) {
      return parsedVersion.major < HISTORICAL_VERSION_THRESHOLD.major;
    }

    return parsedVersion.minor < HISTORICAL_VERSION_THRESHOLD.minor;
  }

  function isVersionToken(value) {
    return PRODUCT_VERSION_RE.test(value || '');
  }

  function getPathLocalePrefix(pathname) {
    const match = (pathname || '').match(/^\/(ru|en)(?=\/|$)/);
    return match ? `/${match[1]}` : '';
  }

  function getDocumentLanguage() {
    const htmlLang = document.documentElement && document.documentElement.lang
      ? document.documentElement.lang.toLowerCase()
      : '';

    if (htmlLang === 'ru' || htmlLang === 'en') {
      return htmlLang;
    }

    return '';
  }

  function getLanguageFromPath(pathname) {
    const match = (pathname || '').match(/^\/(ru|en)(?=\/|$)/);
    return match ? match[1] : '';
  }

  function getLanguageFromDomain() {
    return window.location.hostname.endsWith('.ru') ? 'ru' : 'en';
  }

  function getLocaleInfo(pathname) {
    const language =
      getDocumentLanguage() ||
      getLanguageFromPath(pathname) ||
      getLanguageFromDomain();

    const localeInfo = {
      lang: language,
      localePrefix: getPathLocalePrefix(pathname),
    };

    debugLog('Resolved locale info.', {
      pathname,
      htmlLang: getDocumentLanguage(),
      localeInfo,
    });

    return localeInfo;
  }

  function parseCurrentModulePage(pathname) {
    const localeInfo = getLocaleInfo(pathname);
    let match = pathname.match(/^\/(?:(ru|en)\/)?modules\/([^/]+)(?:\/([^/]+)(?:\/(.*))?)?\/?$/);

    if (match) {
      debugLog('Parsed current page as external module page.', pathname);
      const possibleToken = match[3] || '';
      const hasExplicitToken = CHANNEL_NAMES.has(possibleToken) || PRODUCT_VERSION_RE.test(possibleToken);
      const currentToken = hasExplicitToken ? possibleToken : '';
      const currentTail = hasExplicitToken
        ? normalizeTail(match[4] || '')
        : normalizeTail([possibleToken, match[4] || ''].filter(Boolean).join('/'));

      return {
        localePrefix: localeInfo.localePrefix,
        lang: localeInfo.lang,
        moduleName: match[2],
        currentToken,
        tail: currentTail,
        pageType: 'external',
      };
    }

    match = pathname.match(/^\/(?:(ru|en)\/)?products\/kubernetes-platform\/documentation\/(v\d+\.\d+)\/modules\/([^/]+)(?:\/(.*))?\/?$/);

    if (match) {
      debugLog('Parsed current page as historical module page.', pathname);
      return {
        localePrefix: localeInfo.localePrefix,
        lang: localeInfo.lang,
        moduleName: match[3],
        currentToken: match[2],
        tail: normalizeTail(match[4] || ''),
        pageType: 'historical',
      };
    }

    match = pathname.match(/^\/(?:(ru|en)\/)?products\/kubernetes-platform\/documentation\/modules\/([^/]+)\/(v\d+\.\d+)(?:\/(.*))?\/?$/);

    if (match) {
      debugLog('Parsed current page as legacy historical module page.', pathname);
      return {
        localePrefix: localeInfo.localePrefix,
        lang: localeInfo.lang,
        moduleName: match[2],
        currentToken: match[3],
        tail: normalizeTail(match[4] || ''),
        pageType: 'historical',
      };
    }

    return null;
  }

  function parseCurrentDocumentationPage(pathname) {
    const localeInfo = getLocaleInfo(pathname);
    let match = pathname.match(/^\/(?:(ru|en)\/)?products\/kubernetes-platform\/documentation\/([^/]+)\/(.*)$/);

    if (match && !match[3].startsWith('modules/')) {
      debugLog('Parsed current page as regular documentation page.', pathname);
      return {
        localePrefix: localeInfo.localePrefix,
        lang: localeInfo.lang,
        currentToken: match[2],
        tail: normalizeTail(match[3] || ''),
        pageType: 'docs',
      };
    }

    match = pathname.match(/^\/(?:(ru|en)\/)?documentation\/([^/]+)\/(.*)$/);

    if (match && !match[3].startsWith('modules/')) {
      debugLog('Parsed current page as legacy regular documentation page.', pathname);
      return {
        localePrefix: localeInfo.localePrefix,
        lang: localeInfo.lang,
        currentToken: match[2],
        tail: normalizeTail(match[3] || ''),
        pageType: 'docs',
      };
    }

    return null;
  }

  function extractLinkIntent(linkElement) {
    const resolvedUrl = new URL(linkElement.getAttribute('href'), window.location.href);
    const isHistoricalLink = linkElement.classList.contains('submenu-embedded-link');
    const pathname = resolvedUrl.pathname;
    let match;

    if (isHistoricalLink) {
      match = pathname.match(/^\/(?:(ru|en)\/)?products\/kubernetes-platform\/documentation\/(v\d+\.\d+)\/modules\/([^/]+)(?:\/(.*))?\/?$/);

      if (match) {
        debugLog('Detected historical link intent from absolute historical URL.', pathname);
        return {
          targetType: 'historical',
          token: match[2],
          tail: normalizeTail(match[4] || ''),
          originalUrl: resolvedUrl,
        };
      }

      match = pathname.match(/^\/(?:(ru|en)\/)?products\/kubernetes-platform\/documentation\/modules\/([^/]+)\/(v\d+\.\d+)(?:\/(.*))?\/?$/);

      if (match) {
        debugLog('Detected historical link intent from legacy historical URL.', pathname);
        return {
          targetType: 'historical',
          token: match[3],
          tail: normalizeTail(match[4] || ''),
          originalUrl: resolvedUrl,
        };
      }

      match = pathname.match(/^\/(?:(ru|en)\/)?modules\/[^/]+\/(v\d+\.\d+)(?:\/(.*))?\/?$/);

      if (match) {
        debugLog('Detected historical link intent from relative module URL.', pathname);
        return {
          targetType: 'historical',
          token: match[2],
          tail: normalizeTail(match[3] || ''),
          originalUrl: resolvedUrl,
        };
      }

      match = pathname.match(/^\/(?:(ru|en)\/)?modules\/(v\d+\.\d+)(?:\/(.*))?\/?$/);

      if (match) {
        debugLog('Detected historical link intent from shortened module URL.', pathname);
        return {
          targetType: 'historical',
          token: match[2],
          tail: normalizeTail(match[3] || ''),
          originalUrl: resolvedUrl,
        };
      }

      return null;
    }

    match = pathname.match(/^\/(?:(ru|en)\/)?modules\/[^/]+\/(alpha|beta|early-access|stable|rock-solid|latest)(?:\/(.*))?\/?$/);

    if (match) {
      debugLog('Detected external link intent from explicit channel URL.', pathname);
      return {
        targetType: 'external',
        token: match[2],
        tail: normalizeTail(match[3] || ''),
        originalUrl: resolvedUrl,
      };
    }

    match = pathname.match(/^\/(?:(ru|en)\/)?products\/kubernetes-platform\/documentation\/v\d+\.\d+\/modules\/(alpha|beta|early-access|stable|rock-solid|latest)(?:\/(.*))?\/?$/);

    if (match) {
      debugLog('Detected external link intent from historical page channel URL.', pathname);
      return {
        targetType: 'external',
        token: match[2],
        tail: normalizeTail(match[3] || ''),
        originalUrl: resolvedUrl,
      };
    }

    match = pathname.match(/^\/(?:(ru|en)\/)?modules\/(alpha|beta|early-access|stable|rock-solid|latest)(?:\/(.*))?\/?$/);

    if (match) {
      debugLog('Detected external link intent from shortened stable-like URL.', pathname);
      return {
        targetType: 'external',
        token: match[2],
        tail: normalizeTail(match[3] || ''),
        originalUrl: resolvedUrl,
      };
    }

    match = pathname.match(/^\/(?:(ru|en)\/)?products\/kubernetes-platform\/documentation\/([^/]+)\/(.*)$/);

    if (match && !match[3].startsWith('modules/')) {
      debugLog('Detected documentation link intent from current products URL.', pathname);
      return {
        targetType: 'docs',
        token: match[2],
        tail: normalizeTail(match[3] || ''),
        originalUrl: resolvedUrl,
      };
    }

    match = pathname.match(/^\/(?:(ru|en)\/)?documentation\/([^/]+)\/(.*)$/);

    if (match && !match[3].startsWith('modules/')) {
      debugLog('Detected documentation link intent from legacy documentation URL.', pathname);
      return {
        targetType: 'docs',
        token: match[2],
        tail: normalizeTail(match[3] || ''),
        originalUrl: resolvedUrl,
      };
    }

    return null;
  }

  function buildTargetUrl(currentPage, targetType, token, tail) {
    const localePrefix = currentPage.localePrefix || '';
    const normalizedTail = normalizeTail(tail);

    let basePath = '';

    if (targetType === 'historical') {
      if (isHistoricalVersionBeforeThreshold(token)) {
        basePath = `${localePrefix}/products/kubernetes-platform/documentation/${token}/modules/${currentPage.moduleName}`;
      } else {
        basePath = `${localePrefix}/modules/${currentPage.moduleName}/${token}`;
      }
    } else {
      basePath = `${localePrefix}/modules/${currentPage.moduleName}/${token}`;
    }

    const targetPath = normalizedTail ? `${basePath}/${normalizedTail}` : `${basePath}/`;

    return new URL(targetPath, window.location.origin).toString();
  }

  function buildStableAliasUrl(currentPage, tail) {
    const localePrefix = currentPage.localePrefix || '';
    const normalizedTail = normalizeTail(tail);
    const basePath = `${localePrefix}/modules/${currentPage.moduleName}`;
    const targetPath = normalizedTail ? `${basePath}/${normalizedTail}` : `${basePath}/`;

    return new URL(targetPath, window.location.origin).toString();
  }

  function buildDocumentationVersionUrl(localePrefix, version) {
    const normalizedLocalePrefix = localePrefix || '';
    const targetPath = `${normalizedLocalePrefix}/products/kubernetes-platform/documentation/${version}/`;

    return new URL(targetPath, window.location.origin).toString();
  }

  function getModalTexts(lang) {
    if (lang === 'en') {
      return {
        title: 'Document not found',
        description: 'The selected page was not found.',
        suggestionPrefix: 'Try opening documentation version',
        linkLabel: 'Open documentation version',
        closeLabel: 'Close',
      };
    }

    return {
      title: 'Документ не найден',
      description: 'Не удалось найти страницу документации для выбранной версии.',
      suggestionPrefix: 'Попробуйте перейти на версию документации',
      linkLabel: 'Открыть версию документации',
      closeLabel: 'Закрыть',
    };
  }

  function ensureNotFoundModal() {
    let modal = document.getElementById('doc-version-link-resolver-modal');

    if (modal) {
      return modal;
    }

    const style = document.createElement('style');
    style.id = 'doc-version-link-resolver-modal-style';
    style.textContent = `
      .doc-version-link-resolver-modal {
        display: none;
      }
      .doc-version-link-resolver-modal.is-open {
        display: flex;
      }
      .doc-version-link-resolver-modal .modal-window__wrap {
        width: 550px;
        max-width: calc(100vw - 32px);
        text-align: left;
      }
      .doc-version-link-resolver-modal__title {
        margin: 0 32px 12px 0;
        color: #00122c;
        font-size: 32px;
        line-height: 1.2;
        font-weight: 600;
      }
      .doc-version-link-resolver-modal__description,
      .doc-version-link-resolver-modal__suggestion {
        margin: 0 0 12px;
        font-size: 16px;
        line-height: 1.5;
        color: #6e7084;
      }
      .doc-version-link-resolver-modal__actions {
        display: flex;
        gap: 12px;
        flex-wrap: wrap;
        margin-top: 24px;
      }
      .doc-version-link-resolver-modal__link,
      .doc-version-link-resolver-modal__dismiss {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        text-decoration: none;
      }
      .doc-version-link-resolver-modal__dismiss {
        min-height: auto;
        padding: 0.75em 1.5em;
        border: 2px solid #d3d9e3;
        background: #fff;
        color: #00122c;
        font-weight: 600;
        cursor: pointer;
      }
    `;

    document.head.appendChild(style);

    modal = document.createElement('div');
    modal.id = 'doc-version-link-resolver-modal';
    modal.className = 'modal-window doc-version-link-resolver-modal';
    modal.innerHTML = `
      <div class="modal-window__backdrop" data-doc-version-link-modal-close></div>
      <div class="modal-window__wrap" role="dialog" aria-modal="true" aria-labelledby="doc-version-link-resolver-modal-title">
        <a href="" class="modal-window__close-btn" data-doc-version-link-modal-close aria-label="Close">&times;</a>
        <h2 id="doc-version-link-resolver-modal-title" class="doc-version-link-resolver-modal__title"></h2>
        <p class="doc-version-link-resolver-modal__description"></p>
        <p class="doc-version-link-resolver-modal__suggestion"></p>
        <div class="doc-version-link-resolver-modal__actions">
          <a class="button button_alt doc-version-link-resolver-modal__link" target="_self"></a>
          <button type="button" class="doc-version-link-resolver-modal__dismiss" data-doc-version-link-modal-close></button>
        </div>
      </div>
    `;

    modal.querySelectorAll('[data-doc-version-link-modal-close]').forEach((element) => {
      element.addEventListener('click', function (event) {
        event.preventDefault();
        modal.classList.remove('is-open');
      });
    });

    document.addEventListener('keydown', function (event) {
      if (event.key === 'Escape') {
        modal.classList.remove('is-open');
      }
    });

    document.body.appendChild(modal);
    return modal;
  }

  function resolveSuggestedDocumentationVersion(currentPage, linkIntent) {
    if (isVersionToken(linkIntent.token)) {
      return linkIntent.token;
    }

    if (isVersionToken(currentPage.currentToken)) {
      return currentPage.currentToken;
    }

    return null;
  }

  function showDocumentNotFoundModal(currentPage, linkIntent) {
    const suggestedVersion = resolveSuggestedDocumentationVersion(currentPage, linkIntent);
    const texts = getModalTexts(currentPage.lang);
    const modal = ensureNotFoundModal();
    const title = modal.querySelector('.doc-version-link-resolver-modal__title');
    const description = modal.querySelector('.doc-version-link-resolver-modal__description');
    const suggestion = modal.querySelector('.doc-version-link-resolver-modal__suggestion');
    const link = modal.querySelector('.doc-version-link-resolver-modal__link');
    const dismiss = modal.querySelector('.doc-version-link-resolver-modal__dismiss');

    title.textContent = texts.title;
    description.textContent = texts.description;
    dismiss.textContent = texts.closeLabel;

    if (suggestedVersion) {
      suggestion.textContent = `${texts.suggestionPrefix} ${suggestedVersion}.`;
      suggestion.style.display = '';
      link.textContent = `${texts.linkLabel} ${suggestedVersion}`;
      link.href = buildDocumentationVersionUrl(currentPage.localePrefix, suggestedVersion);
      link.style.display = '';
    } else {
      debugLog('No documentation version available for modal fallback.', {
        currentPage,
        linkIntent,
      });
      suggestion.textContent = '';
      suggestion.style.display = 'none';
      link.removeAttribute('href');
      link.style.display = 'none';
    }

    debugLog('Showing not-found modal.', {
      suggestedVersion,
      documentationVersionUrl: suggestedVersion ? link.href : null,
    });

    modal.classList.add('is-open');
  }

  function buildCandidateUrls(linkElement, currentPage, linkIntent) {
    const candidateUrls = [];
    const seenUrls = new Set();
    const candidateTails = [
      linkIntent.tail,
      currentPage.tail,
      '',
      'readme.html',
      'index.html',
    ];

    function addCandidate(candidateUrl) {
      if (!candidateUrl) {
        return;
      }

      if (seenUrls.has(candidateUrl)) {
        return;
      }

      seenUrls.add(candidateUrl);
      candidateUrls.push(candidateUrl);
    }

    addCandidate(linkIntent.originalUrl.toString());

    if (linkIntent.targetType === 'docs' || currentPage.pageType === 'docs') {
      debugLog('Using direct link validation for regular documentation page.', {
        originalHref: linkElement.href,
      });
      return candidateUrls;
    }

    candidateTails.forEach((tail) => {
      if (linkIntent.targetType === 'external' && linkIntent.token === 'stable') {
        addCandidate(buildStableAliasUrl(currentPage, tail));
      }

      addCandidate(buildTargetUrl(currentPage, linkIntent.targetType, linkIntent.token, tail));
    });

    debugLog('Built candidate URLs.', {
      originalHref: linkElement.href,
      currentPage,
      linkIntent,
      candidateUrls,
    });

    return candidateUrls;
  }

  async function requestPage(url, method) {
    debugLog('Checking candidate URL.', { method, url });
    const response = await fetch(url, {
      method,
      credentials: 'same-origin',
      redirect: 'follow',
      cache: 'no-store',
    });

    if (response.ok) {
      return true;
    }

    if (response.status === 404) {
      return false;
    }

    return null;
  }

  async function isUrlReachable(url) {
    const cacheKey = new URL(url).pathname;

    if (Object.prototype.hasOwnProperty.call(availabilityCache, cacheKey)) {
      debugLog('Using cached availability result.', { url, reachable: availabilityCache[cacheKey] });
      return availabilityCache[cacheKey];
    }

    let isReachable = false;

    try {
      const headResult = await requestPage(url, 'HEAD');

      if (headResult === true || headResult === false) {
        isReachable = headResult;
      } else {
        const getResult = await requestPage(url, 'GET');
        isReachable = getResult === true;
      }
    } catch (error) {
      try {
        const getResult = await requestPage(url, 'GET');
        isReachable = getResult === true;
      } catch (fallbackError) {
        isReachable = false;
      }
    }

    availabilityCache[cacheKey] = isReachable;
    saveAvailabilityCache();
    debugLog('Stored availability result.', { url, reachable: isReachable });

    return isReachable;
  }

  function shouldSkipInterception(event, linkElement) {
    if (!linkElement) {
      return true;
    }

    if (event.defaultPrevented || event.button !== 0) {
      return true;
    }

    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
      return true;
    }

    if (linkElement.target && linkElement.target !== '_self') {
      return true;
    }

    if (linkElement.hasAttribute('download')) {
      return true;
    }

    return false;
  }

  async function handleMenuLinkClick(event) {
    const linkElement = event.target.closest(MENU_LINK_SELECTOR);

    if (shouldSkipInterception(event, linkElement)) {
      return;
    }

    const currentPage =
      parseCurrentModulePage(window.location.pathname) ||
      parseCurrentDocumentationPage(window.location.pathname);

    if (!currentPage) {
      debugLog('Skipping click interception because current page format is unsupported.', window.location.pathname);
      return;
    }

    const linkIntent = extractLinkIntent(linkElement);

    if (!linkIntent) {
      debugLog('Skipping click interception because link intent was not recognized.', linkElement.href);
      return;
    }

    if (linkIntent.targetType !== 'docs' && !CHANNEL_NAMES.has(linkIntent.token) && !PRODUCT_VERSION_RE.test(linkIntent.token)) {
      debugLog('Skipping click interception because token is unsupported.', linkIntent.token);
      return;
    }

    event.preventDefault();
    debugLog('Intercepted version menu click.', {
      currentPath: window.location.pathname,
      originalHref: linkElement.href,
    });

    const candidateUrls = buildCandidateUrls(linkElement, currentPage, linkIntent);

    for (const candidateUrl of candidateUrls) {
      if (await isUrlReachable(candidateUrl)) {
        debugLog('Navigating to resolved candidate URL.', candidateUrl);
        if (candidateUrl !== window.location.href) {
          window.location.assign(candidateUrl);
        }

        return;
      }
    }

    console.warn('No valid module version link candidates found.', {
      currentPath: window.location.pathname,
      originalHref: linkElement.href,
      candidates: candidateUrls,
    });
    showDocumentNotFoundModal(currentPage, linkIntent);
  }

  debugLog('Initialized resolver.', { debugEnabled });

  document.addEventListener('click', function (event) {
    void handleMenuLinkClick(event);
  });
})();

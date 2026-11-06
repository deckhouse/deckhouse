/*
 * Glossary tooltips.
 *
 * Links inside the page content that point to the glossary page with an anchor
 * (for example "../glossary.html#project") get the "glossary-term" class and a
 * tippy tooltip with the short definition of the term. Definitions come from the
 * JSON published next to the glossary page (glossary.json) as HTML rendered from
 * inline Markdown, fetched lazily on the first hover or focus of such a link, or
 * when the browser is idle after load.
 *
 * Plain static JS: no templating engine logic, works for Jekyll and Hugo pages.
 */
(function () {
  'use strict';

  var GLOSSARY_PAGE = /\/glossary\.html$/;
  var CONTENT_LINKS = '.post-content a[href]';

  // Same options as the "[title]" tooltips initialised in head-site.html, plus
  // "interactive": the description is HTML and may contain links to click.
  var TIPPY_OPTIONS = {
    maxWidth: 400,
    allowHTML: true,
    interactive: true,
    interactiveDebounce: 75,
    appendTo: function () { return document.body; },
    theme: 'light',
    arrow: '<svg width="14" height="10" viewBox="0 0 14 10" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M7 9L13 1L1 1L7 9Z" fill="#EFF3F9"/><path d="M13 1L7 9L1 1" stroke="#C4CAD4" stroke-linecap="round"/></svg>'
  };

  // The glossary page itself links its own terms; no tooltips there.
  if (GLOSSARY_PAGE.test(window.location.pathname)) {
    return;
  }

  var jsonPromises = {};

  function jsonUrl(link) {
    return link.pathname.replace(/\.html$/, '.json');
  }

  function termId(link) {
    try {
      return decodeURIComponent(link.hash.slice(1));
    } catch (e) {
      return link.hash.slice(1);
    }
  }

  function fetchGlossary(url) {
    if (!jsonPromises[url]) {
      jsonPromises[url] = fetch(url, { credentials: 'same-origin' }).then(function (response) {
        if (!response.ok) {
          throw new Error('Glossary request failed: ' + response.status);
        }
        return response.json();
      });
    }
    return jsonPromises[url];
  }

  function findGlossaryLinks() {
    var links = document.querySelectorAll(CONTENT_LINKS);
    var result = [];
    for (var i = 0; i < links.length; i++) {
      var link = links[i];
      if (GLOSSARY_PAGE.test(link.pathname) && link.hash.length > 1) {
        result.push(link);
      }
    }
    return result;
  }

  function groupByJson(links) {
    var groups = {};
    links.forEach(function (link) {
      var url = jsonUrl(link);
      (groups[url] = groups[url] || []).push(link);
    });
    return groups;
  }

  // The description is HTML rendered next to the glossary page, so its relative
  // links are relative to that page, not to the page the tooltip is shown on.
  function absoluteLinks(html, baseUrl) {
    var container = document.createElement('div');
    container.innerHTML = html;
    var anchors = container.querySelectorAll('a[href]');
    for (var i = 0; i < anchors.length; i++) {
      anchors[i].setAttribute('href', new URL(anchors[i].getAttribute('href'), baseUrl).href);
    }
    return container.innerHTML;
  }

  function createTooltips(links, glossary) {
    var descriptions = new WeakMap();
    var targets = [];
    links.forEach(function (link) {
      var term = glossary[termId(link)];
      if (term && typeof term.description === 'string' && term.description.trim() !== '') {
        descriptions.set(link, absoluteLinks(term.description, link.href));
        targets.push(link);
      }
    });
    if (targets.length === 0) {
      return;
    }
    var options = Object.assign({}, TIPPY_OPTIONS, {
      content: function (reference) {
        return descriptions.get(reference);
      }
    });
    var instances = window.tippy(targets, options);
    // The link that triggered loading is still hovered or focused: show its
    // tooltip now instead of waiting for the next pointer or focus event.
    instances.forEach(function (instance) {
      var el = instance.reference;
      if (el === document.activeElement || el.matches(':hover')) {
        instance.show();
      }
    });
  }

  function init() {
    var links = findGlossaryLinks();
    if (links.length === 0) {
      return;
    }

    links.forEach(function (link) {
      link.classList.add('glossary-term');
    });

    if (typeof window.tippy !== 'function' || typeof window.fetch !== 'function') {
      return;
    }

    var started = false;
    var groups = groupByJson(links);

    function load() {
      if (started) {
        return;
      }
      started = true;
      links.forEach(function (link) {
        link.removeEventListener('mouseenter', load);
        link.removeEventListener('focus', load);
      });
      Object.keys(groups).forEach(function (url) {
        fetchGlossary(url)
          .then(function (glossary) {
            createTooltips(groups[url], glossary);
          })
          .catch(function () {
            // Definitions are optional: the link still leads to the glossary.
          });
      });
    }

    links.forEach(function (link) {
      link.addEventListener('mouseenter', load);
      link.addEventListener('focus', load);
    });

    if (typeof window.requestIdleCallback === 'function') {
      window.addEventListener('load', function () {
        window.requestIdleCallback(load);
      });
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();

// PDF export for documentation pages.
//
// Enabled only on pages that set `allowPDFDownload: true` in their front matter
// (the button and the pdfmake/html-to-pdfmake libraries are rendered/loaded only
// in that case). Builds a client-side PDF from the current page content using a
// single fixed template: a title page plus running header/footer.
//
// Depends on the globals provided by the libraries loaded in head-site.html:
//   - pdfMake        (pdfmake.min.js + vfs_fonts.js)
//   - htmlToPdfmake  (html-to-pdfmake.min.js)

(function () {
  'use strict';

  var LANG = (document.documentElement.getAttribute('lang') || 'en').toLowerCase();
  var IS_RU = LANG.indexOf('ru') === 0;

  // Localized strings used inside the generated PDF (the button label itself
  // comes from Jekyll i18n; these are only for PDF-internal text).
  var STRINGS = {
    generatedAt: IS_RU ? 'Сформировано' : 'Generated',
    source: IS_RU ? 'Источник' : 'Source',
    page: IS_RU ? 'Стр.' : 'Page',
    building: IS_RU ? 'Формирование PDF…' : 'Building PDF…'
  };

  function pad(n) {
    return n < 10 ? '0' + n : '' + n;
  }

  function formatDate(d) {
    // YYYY-MM-DD HH:MM — locale-neutral, unambiguous.
    return (
      d.getFullYear() +
      '-' + pad(d.getMonth() + 1) +
      '-' + pad(d.getDate()) +
      ' ' + pad(d.getHours()) +
      ':' + pad(d.getMinutes())
    );
  }

  function slugify(text) {
    var s = (text || 'document')
      .toLowerCase()
      .replace(/[^a-z0-9а-яё]+/gi, '-')
      .replace(/^-+|-+$/g, '');
    return s || 'document';
  }

  function getTitle() {
    var el = document.querySelector('.docs__title');
    var title = el ? el.textContent.trim() : document.title;
    return title || document.title || 'Document';
  }

  // Clone the content node and strip elements that should not appear in the PDF
  // (interactive controls, injected buttons, scripts, etc.).
  function getContentClone() {
    var source = document.querySelector('.post-content');
    if (!source) {
      return null;
    }
    var clone = source.cloneNode(true);

    var dropSelectors = [
      'script',
      'style',
      'noscript',
      '.pdf-download',
      '#pdf-download-button',
      '.githubEditButton',
      '.toc',
      '#toc',
      '.anchorjs-link',
      'button.copy',
      '.copybtn',
      '.tags',
      // Interactive code-block controls: expand/collapse and copy buttons.
      // Their SVG icons are UI chrome, not content, and their nesting inside
      // <pre>/<button> stacks makes pdfmake choke ("unsupported number: NaN").
      // NOTE: do not drop `.code__transfer` — that class sits on the <pre>
      // itself, so removing it would delete the whole code block.
      '.wrap__button',
      '.wrap__icon',
      '.copy-code',
      '.copy-button',
      // Line-number gutters inside code blocks (not useful in a PDF).
      '.line-number',
      // Alert/callout icons (info/warning) — decorative sprite icons.
      '.alert__icon'
    ];
    clone.querySelectorAll(dropSelectors.join(',')).forEach(function (node) {
      node.parentNode && node.parentNode.removeChild(node);
    });

    dropUnsupportedSvg(clone);
    absolutizeLinks(clone);

    return clone;
  }

  // Remove inline <svg> that pdfmake cannot render. Sprite icons use
  // <use xlink:href="#..."> with no intrinsic geometry, which makes pdfmake's
  // vector renderer produce NaN. Such SVGs carry no meaningful content in the
  // PDF, so they are dropped; self-contained SVGs (with real shapes) are kept.
  function dropUnsupportedSvg(root) {
    root.querySelectorAll('svg').forEach(function (svg) {
      var hasShapes = svg.querySelector('path,rect,circle,ellipse,polygon,polyline,line');
      var usesSprite = svg.querySelector('use');
      if (usesSprite || !hasShapes) {
        svg.parentNode && svg.parentNode.removeChild(svg);
      }
    });
  }

  // Rewrite relative link targets to absolute URLs so they work in the
  // generated PDF (which has no page origin to resolve against). Pure fragment
  // links (in-document anchors like "#install") are left untouched.
  function absolutizeLinks(root) {
    root.querySelectorAll('a[href]').forEach(function (a) {
      var raw = a.getAttribute('href');
      // Leave in-document anchors (pure fragments) as-is; absolutize everything
      // else. `a.href` is the browser-resolved absolute URL of the raw href.
      if (!raw || raw.charAt(0) === '#') {
        return;
      }
      a.setAttribute('href', a.href);
    });
  }

  // pdfmake (browser) cannot fetch images by URL, and its inline-SVG renderer
  // chokes on many real-world SVGs (missing pixel sizes, unsupported units,
  // ellipse/circle geometry -> "unsupported number: NaN"). So EVERY image,
  // including SVG, is rasterized to a PNG data URL on a canvas before pdfmake
  // runs. Images that fail to load / rasterize (404, CORS taint) are dropped so
  // one bad asset doesn't fail the whole export.
  // Returns a promise that resolves once all images are processed.
  function inlineImages(root) {
    var imgs = Array.prototype.slice.call(root.querySelectorAll('img[src]'));
    return Promise.all(imgs.map(function (img) {
      return inlineOneImage(img);
    }));
  }

  function isSvgUrl(url) {
    return /\.svg(\?|#|$)/i.test(url) || url.indexOf('data:image/svg') === 0;
  }

  function drop(node) {
    if (node && node.parentNode) {
      node.parentNode.removeChild(node);
    }
  }

  // Printable content width on A4 (595pt) minus the 40pt side margins, in pt.
  var MAX_IMAGE_WIDTH = 515;

  function inlineOneImage(img) {
    var src = img.getAttribute('src');
    if (!src) {
      drop(img);
      return Promise.resolve();
    }

    var absolute = img.src; // browser-resolved absolute URL
    var svg = isSvgUrl(src) || isSvgUrl(absolute);

    return rasterToDataUrl(absolute, svg, img)
      .then(function (res) {
        img.setAttribute('src', res.dataUrl);
        // html-to-pdfmake copies width/height ATTRIBUTES into the pdfmake image
        // node. Non-numeric values ("100%", "auto") become NaN and crash the
        // image transform. Replace them with clean numeric pixel dimensions,
        // capped to the printable width so wide images don't overflow.
        applyPdfImageSize(img, res.w, res.h);
        img.removeAttribute('srcset');
      })
      .catch(function () {
        drop(img);
      });
  }

  // Set numeric width/height attributes that pdfmake can consume, scaling down
  // proportionally to MAX_IMAGE_WIDTH when the source is wider.
  function applyPdfImageSize(img, w, h) {
    var width = w;
    var height = h;
    if (width > MAX_IMAGE_WIDTH) {
      height = Math.round(height * (MAX_IMAGE_WIDTH / width));
      width = MAX_IMAGE_WIDTH;
    }
    img.setAttribute('width', width);
    img.setAttribute('height', height);
    img.removeAttribute('style'); // strip CSS sizing (%, auto, em, ...)
  }

  // Load `url` into an Image and paint it onto a canvas, returning
  // { dataUrl, w, h } with the rasterized pixel size. For SVG sources the
  // intrinsic size is often absent, so an explicit target size is resolved from
  // the <img>/SVG attributes with a sane fallback.
  function rasterToDataUrl(url, isSvg, srcImg) {
    return new Promise(function (resolve, reject) {
      var image = new Image();
      image.crossOrigin = 'anonymous';

      image.onload = function () {
        try {
          var w = image.naturalWidth || image.width || 0;
          var h = image.naturalHeight || image.height || 0;

          if ((!w || !h) && isSvg) {
            var size = fallbackSvgSize(srcImg);
            w = size.w;
            h = size.h;
          }

          w = Math.round(w);
          h = Math.round(h);
          if (!w || !h) {
            reject(new Error('zero-sized image'));
            return;
          }

          var canvas = document.createElement('canvas');
          canvas.width = w;
          canvas.height = h;
          canvas.getContext('2d').drawImage(image, 0, 0, w, h);
          resolve({ dataUrl: canvas.toDataURL('image/png'), w: w, h: h });
        } catch (e) {
          reject(e); // tainted canvas (CORS) or other draw error
        }
      };
      image.onerror = function () {
        reject(new Error('image load error'));
      };
      image.src = url;
    });
  }

  // Walk the pdfmake content tree and fix every image node in place. An "image"
  // node is any object with an `image` string property. We recompute width from
  // the PNG's real pixel size (capped to the printable width) and strip any
  // height/fit that could carry a NaN.
  function normalizeImageNodes(node) {
    if (!node || typeof node !== 'object') {
      return;
    }
    if (Array.isArray(node)) {
      node.forEach(normalizeImageNodes);
      return;
    }

    if (typeof node.image === 'string') {
      if (node.image.indexOf('data:image/') !== 0) {
        // Not an inlined data URL (pdfmake can't fetch it) -> neutralize the
        // node so it renders as nothing instead of crashing.
        delete node.image;
        delete node.width;
        delete node.height;
        delete node.fit;
        node.text = '';
      } else {
        var dims = pngDataUrlSize(node.image);
        var width = dims ? dims.w : MAX_IMAGE_WIDTH;
        if (width > MAX_IMAGE_WIDTH) {
          width = MAX_IMAGE_WIDTH;
        }
        node.width = width;    // valid number pdfmake can always render
        delete node.height;    // let pdfmake keep aspect ratio from width
        delete node.fit;       // fit with any NaN would also crash
      }
    }

    // Recurse into common pdfmake container keys and any nested objects.
    Object.keys(node).forEach(function (key) {
      var value = node[key];
      if (value && typeof value === 'object') {
        normalizeImageNodes(value);
      }
    });
  }

  // Colored-square status emojis (used in comparison/matrix tables) that the
  // Roboto vfs font can't render — they show up as empty "tofu" boxes. They are
  // replaced with a font-independent vector square (pdfmake `canvas`) in the
  // matching color, drawn via drawSquare() below.
  var EMOJI_SQUARES = {
    '🟩': '#2fa361', // 🟩 green
    '🟨': '#e8a33d', // 🟨 yellow
    '🟥': '#d64541', // 🟥 red
    '🟫': '#8a6d3b', // 🟫 brown
    '🟦': '#0066ff', // 🟦 blue
    '🟧': '#e8730c', // 🟧 orange
    '🟪': '#8e44ad', // 🟪 purple
    '⬛': '#333333',       // ⬛ black
    '⬜': '#bbbbbb'        // ⬜ white
  };
  var EMOJI_SQUARE_RE = /(\uD83D[\uDFE5-\uDFEB]|⬛|⬜)/g;
  var SQUARE_SIZE = 9; // pt

  function drawSquare(color) {
    return {
      canvas: [{ type: 'rect', x: 0, y: 0, w: SQUARE_SIZE, h: SQUARE_SIZE, color: color }],
      margin: [0, 1, 0, 0]
    };
  }

  // Walk the pdfmake tree and replace unrenderable status-square emojis. Roboto
  // has no glyph for them (they render as tofu), so a text node that is exactly
  // one such emoji is turned into a colored vector square. Any emoji left inside
  // a longer string (no known cases today) is stripped so no tofu remains.
  function replaceEmojiSquares(node) {
    if (!node || typeof node !== 'object') {
      return;
    }
    if (Array.isArray(node)) {
      node.forEach(replaceEmojiSquares);
      return;
    }

    if (typeof node.text === 'string') {
      var trimmed = node.text.trim();
      if (EMOJI_SQUARES.hasOwnProperty(trimmed)) {
        // Whole cell/run is a single status square -> draw it as a vector.
        delete node.text;
        node.canvas = drawSquare(EMOJI_SQUARES[trimmed]).canvas;
        node.margin = [0, 1, 0, 0];
      } else if (EMOJI_SQUARE_RE.test(node.text)) {
        // Mixed content: drop the emoji glyphs, keep the surrounding text.
        node.text = node.text.replace(EMOJI_SQUARE_RE, '').replace(/\s{2,}/g, ' ');
      }
    }

    Object.keys(node).forEach(function (key) {
      var value = node[key];
      if (value && typeof value === 'object') {
        replaceEmojiSquares(value);
      }
    });
  }

  // Read the pixel dimensions of a base64 PNG data URL from its IHDR chunk,
  // without decoding the image. Returns { w, h } or null if it can't be parsed.
  function pngDataUrlSize(dataUrl) {
    try {
      var comma = dataUrl.indexOf(',');
      if (comma < 0 || dataUrl.indexOf('image/png') < 0) {
        return null;
      }
      var bytes = atob(dataUrl.slice(comma + 1));
      // PNG signature (8 bytes) + IHDR length (4) + "IHDR" (4) -> width at
      // offset 16, height at offset 20, each a big-endian uint32.
      var w =
        (bytes.charCodeAt(16) << 24) |
        (bytes.charCodeAt(17) << 16) |
        (bytes.charCodeAt(18) << 8) |
        bytes.charCodeAt(19);
      var h =
        (bytes.charCodeAt(20) << 24) |
        (bytes.charCodeAt(21) << 16) |
        (bytes.charCodeAt(22) << 8) |
        bytes.charCodeAt(23);
      if (!w || !h) {
        return null;
      }
      return { w: w >>> 0, h: h >>> 0 };
    } catch (e) {
      return null;
    }
  }

  // Best-effort pixel size for an SVG that has no intrinsic dimensions: use the
  // rendered <img> box, then explicit width/height attributes, else a default.
  function fallbackSvgSize(img) {
    var w = (img && (img.getAttribute('width') || img.clientWidth)) || 0;
    var h = (img && (img.getAttribute('height') || img.clientHeight)) || 0;
    w = parseInt(w, 10) || 0;
    h = parseInt(h, 10) || 0;
    if (w && !h) { h = w; }
    if (h && !w) { w = h; }
    if (!w || !h) { w = h = 64; }
    return { w: w, h: h };
  }

  // Alert box color scheme (fill + left accent bar) keyed by callout kind.
  var ALERT_COLORS = {
    info:    { fill: '#eef4ff', bar: '#0066ff' },
    warning: { fill: '#fff7e6', bar: '#e8a33d' },
    danger:  { fill: '#fdeceb', bar: '#d64541' },
    tip:     { fill: '#ecf8f0', bar: '#2fa361' }
  };

  // Heading styling. `top`/`bottom` are the margins (in pt) added above / kept
  // below each heading — html-to-pdfmake gives headings only a small marginBottom
  // and no marginTop, so they crowd the text above and float far from the text
  // they introduce. `size` mirrors the site's heading scale (site px * ~0.75 pt),
  // keeping a clear h1 > h2 > h3 hierarchy that pdfmake otherwise flattens.
  var HEADING_STYLE = {
    h1: { top: 12, bottom: 3, size: 27, bold: true },
    h2: { top: 12, bottom: 3, size: 18, bold: true },
    h3: { top: 10, bottom: 3, size: 15, bold: true },
    h4: { top: 10, bottom: 3, size: 13, bold: true },
    h5: { top: 10, bottom: 3, size: 11, bold: true },
    h6: { top: 10, bottom: 3, size: 10, bold: true }
  };

  // Build a tinted callout box with a colored left accent bar around `ret`.
  function alertBox(ret, scheme) {
    return {
      // Keep the whole callout together: pdfmake moves an unbreakable block to
      // the next page rather than splitting it across the page boundary.
      unbreakable: true,
      table: {
        widths: ['*'],
        body: [[{ stack: [ret], fillColor: scheme.fill, margin: [8, 6, 8, 6] }]]
      },
      layout: {
        hLineWidth: function () { return 0; },
        vLineWidth: function (i) { return i === 0 ? 3 : 0; },
        vLineColor: function () { return scheme.bar; },
        paddingLeft: function () { return 8; },
        paddingRight: function () { return 8; },
        paddingTop: function () { return 4; },
        paddingBottom: function () { return 4; }
      },
      margin: [0, 6, 0, 6]
    };
  }

  // Per-element hook for html-to-pdfmake. Renders visual containers that the
  // library otherwise flattens into plain text, and tunes heading spacing:
  //   - .alert__wrap  -> a tinted box with a colored left accent bar;
  //   - <blockquote>  -> the same box, styled as an info callout;
  //   - <pre>         -> a light-gray code panel;
  //   - <h1>..<h6>    -> more space before, less after.
  function customTag(params) {
    var el = params.element;
    var ret = params.ret;
    var tag = (el && el.nodeName ? el.nodeName : '').toLowerCase();

    if (tag === 'div' && el.classList && el.classList.contains('alert__wrap')) {
      var kind = 'info';
      ['info', 'warning', 'danger', 'tip'].forEach(function (k) {
        if (el.classList.contains(k)) { kind = k; }
      });
      return alertBox(ret, ALERT_COLORS[kind] || ALERT_COLORS.info);
    }

    if (tag === 'blockquote') {
      return alertBox(ret, ALERT_COLORS.info);
    }

    if (tag === 'pre') {
      return {
        table: {
          widths: ['*'],
          body: [[{ stack: [ret], fillColor: '#f6f8fa', margin: [8, 6, 8, 6] }]]
        },
        layout: 'noBorders',
        margin: [0, 6, 0, 6]
      };
    }

    if (HEADING_STYLE[tag]) {
      var m = HEADING_STYLE[tag];
      // pdfmake's `margin` is [left, top, right, bottom]; overriding it also
      // clears html-to-pdfmake's marginTop/marginBottom on this node.
      ret.margin = [0, m.top, 0, m.bottom];
      delete ret.marginTop;
      delete ret.marginBottom;
      // Enforce the site-matched size/weight hierarchy.
      ret.fontSize = m.size;
      ret.bold = m.bold;
      // Tag headings so the pageBreakBefore callback can keep them from being
      // split across pages or stranded at the very bottom of a page.
      ret.headlineLevel = 1;
      return ret;
    }

    return ret;
  }

  function buildDocDefinition(title, contentClone) {
    var now = new Date();
    var url = window.location.href;

    var content = htmlToPdfmake(contentClone.innerHTML, {
      window: window,
      // Keep the page structure readable; drop authored inline colors/sizes so
      // the fixed template styling wins.
      removeExtraBlanks: true,
      ignoreStyles: ['color', 'background', 'background-color', 'font-size', 'line-height'],
      // The pdfmake browser build only ships Roboto in its vfs; the standard
      // Courier font isn't usable (missing metrics -> crash). So code isn't set
      // in a monospace face — instead inline code gets a light fill and code
      // blocks get a gray panel (via customTag) to read as code.
      defaultStyles: {
        code: { background: '#f2f4f7' },
        pre: { fontSize: 8.5, preserveLeadingSpaces: true }
      },
      customTag: customTag
    });

    // Definitive guard: whatever width/height html-to-pdfmake derived from the
    // markup, normalize every image node to a valid numeric width computed from
    // the actual PNG bytes. This prevents pdfmake's "unsupported number: NaN"
    // crash in renderImage when a bad/absent dimension slips through.
    normalizeImageNodes(content);

    // Swap unrenderable status-square emojis (e.g. in comparison tables) for
    // colored ■ glyphs the bundled font can actually draw.
    replaceEmojiSquares(content);

    // Title page followed by the converted content on a new page.
    var titlePage = [
      { text: 'Deckhouse', style: 'brand', margin: [0, 160, 0, 0] },
      { text: title, style: 'coverTitle', margin: [0, 24, 0, 0] },
      {
        text: STRINGS.source + ': ' + url,
        style: 'coverMeta',
        link: url,
        margin: [0, 40, 0, 0]
      },
      {
        text: STRINGS.generatedAt + ': ' + formatDate(now),
        style: 'coverMeta',
        margin: [0, 4, 0, 0]
      },
      { text: '', pageBreak: 'after' }
    ];

    return {
      info: {
        title: title,
        creator: 'Deckhouse documentation'
      },
      pageSize: 'A4',
      pageMargins: [40, 60, 40, 60],
      content: titlePage.concat(content),
      // Keep a heading with the content it introduces: push it to the next page
      // when it would otherwise be split across the page boundary or left as the
      // last thing on a page with nothing following it.
      pageBreakBefore: function (currentNode, followingNodesOnPage) {
        if (!currentNode.headlineLevel) {
          return false;
        }
        var splitAcrossPages =
          currentNode.pageNumbers && currentNode.pageNumbers.length > 1;
        var strandedAtBottom =
          !followingNodesOnPage || followingNodesOnPage.length === 0;
        return splitAcrossPages || strandedAtBottom;
      },
      header: function (currentPage) {
        // No header on the cover page.
        if (currentPage === 1) {
          return null;
        }
        return {
          text: title,
          style: 'runningHead',
          margin: [40, 20, 40, 0]
        };
      },
      footer: function (currentPage, pageCount) {
        if (currentPage === 1) {
          return null;
        }
        return {
          columns: [
            { text: url, style: 'runningFoot', link: url, width: '*' },
            {
              text: STRINGS.page + ' ' + currentPage + ' / ' + pageCount,
              style: 'runningFoot',
              alignment: 'right',
              width: 'auto'
            }
          ],
          margin: [40, 10, 40, 0]
        };
      },
      defaultStyle: {
        fontSize: 10,
        lineHeight: 1.3
      },
      styles: {
        brand: { fontSize: 28, bold: true, color: '#0066ff' },
        coverTitle: { fontSize: 22, bold: true, color: '#111111' },
        coverMeta: { fontSize: 10, color: '#555555' },
        runningHead: { fontSize: 8, color: '#999999' },
        runningFoot: { fontSize: 8, color: '#999999' }
      }
    };
  }

  function onClick(button) {
    if (typeof pdfMake === 'undefined' || typeof htmlToPdfmake === 'undefined') {
      // Libraries failed to load; nothing we can do client-side.
      return;
    }

    var contentClone = getContentClone();
    if (!contentClone) {
      return;
    }

    var title = getTitle();

    button.disabled = true;
    button.classList.add('is-loading');

    var done = function () {
      button.disabled = false;
      button.classList.remove('is-loading');
    };

    // Images must be inlined (fetched/rasterized) before pdfmake runs, since it
    // cannot resolve image URLs in the browser.
    inlineImages(contentClone)
      .then(function () {
        var docDefinition = buildDocDefinition(title, contentClone);
        pdfMake.createPdf(docDefinition).download(slugify(title) + '.pdf');
        done();
      })
      .catch(function (e) {
        if (window.console && console.error) {
          console.error('PDF export failed:', e);
        }
        done();
      });
  }

  function init() {
    var button = document.getElementById('pdf-download-button');
    if (!button) {
      return;
    }
    button.addEventListener('click', function () {
      onClick(button);
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();

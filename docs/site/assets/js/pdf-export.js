// PDF export for documentation pages.
//
// Enabled only on pages that set `allowPDFDownload: true` in their front matter
// (this small handler is loaded there). Builds a client-side PDF from the
// current page content using a single fixed template: a title page plus running
// header/footer.
//
// The heavy libraries it relies on are NOT loaded with the page — they are
// fetched lazily on the first download click (see loadPdfLibraries):
//   - pdfMake        (pdfmake.min.js)
//   - htmlToPdfmake  (html-to-pdfmake.min.js)
//   - DejaVu Sans    (TTF faces under /assets/fonts/dejavu/, registered in vfs)
//
// DejaVu is the default PDF font (not Roboto): it covers Cyrillic, box-drawing,
// and common symbols such as ✔ that Roboto lacks. Faces are fetched as binary
// and injected into pdfMake.vfs at runtime — no prebuilt vfs_fonts.js needed.

(function () {
  'use strict';

  // Lazy-loaded library filenames, in dependency order.
  var PDF_LIBS = ['pdfmake.min.js', 'html-to-pdfmake.min.js'];

  // Directory + cache-busting version for the lazy libraries, derived from this
  // script's own <script src> so they match the built asset hash.
  var ASSET_BASE = (function () {
    var self = document.currentScript ||
      document.querySelector('script[src*="pdf-export.js"]');
    var src = self ? self.getAttribute('src') : '/assets/js/pdf-export.js';
    var version = self ? self.getAttribute('data-pdf-assets-version') : '';
    var dir = src.replace(/[?#].*$/, '').replace(/\/[^/]*$/, '/'); // strip filename
    return { dir: dir || '/assets/js/', query: version ? '?v=' + version : '' };
  })();

  // DejaVu Sans faces used as the document default font. Filenames must match
  // files under /assets/fonts/dejavu/. pdfmake requires all four style keys.
  var DEJAVU_FONT_DIR = '/assets/fonts/dejavu/';
  var DEJAVU_FONT_FILES = {
    normal: 'DejaVuSans.ttf',
    bold: 'DejaVuSans-Bold.ttf',
    italics: 'DejaVuSans-Oblique.ttf',
    bolditalics: 'DejaVuSans-BoldOblique.ttf'
  };
  var DEFAULT_PDF_FONT = 'DejaVu';

  var librariesPromise = null;

  // Load a single script once; resolves when it has executed.
  function loadScript(url) {
    return new Promise(function (resolve, reject) {
      var el = document.createElement('script');
      el.src = url;
      el.async = false; // preserve execution order across sequential loads
      el.onload = function () { resolve(); };
      el.onerror = function () { reject(new Error('Failed to load ' + url)); };
      document.head.appendChild(el);
    });
  }

  // pdfmake's vfs stores fonts as base64 strings. Chunked fromCharCode avoids
  // call-stack limits on large TTF files (~0.6–0.7MB each).
  function arrayBufferToBase64(buffer) {
    var bytes = new Uint8Array(buffer);
    var chunk = 0x8000;
    var binary = '';
    for (var i = 0; i < bytes.length; i += chunk) {
      binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunk));
    }
    return btoa(binary);
  }

  // Fetch DejaVu TTF faces, put them into pdfMake.vfs, and register the family.
  // Idempotent: subsequent calls reuse the same promise / already-registered font.
  function loadDejaVuFonts() {
    if (pdfMake.fonts && pdfMake.fonts[DEFAULT_PDF_FONT] &&
        pdfMake.vfs && pdfMake.vfs[DEJAVU_FONT_FILES.normal]) {
      return Promise.resolve();
    }

    pdfMake.vfs = pdfMake.vfs || {};

    var filenames = [];
    Object.keys(DEJAVU_FONT_FILES).forEach(function (style) {
      var name = DEJAVU_FONT_FILES[style];
      if (filenames.indexOf(name) === -1) {
        filenames.push(name);
      }
    });

    return Promise.all(filenames.map(function (filename) {
      if (pdfMake.vfs[filename]) {
        return Promise.resolve();
      }
      var url = DEJAVU_FONT_DIR + filename + ASSET_BASE.query;
      return fetch(url).then(function (res) {
        if (!res.ok) {
          throw new Error('Failed to load font ' + filename + ' (' + res.status + ')');
        }
        return res.arrayBuffer();
      }).then(function (buf) {
        pdfMake.vfs[filename] = arrayBufferToBase64(buf);
      });
    })).then(function () {
      pdfMake.fonts = pdfMake.fonts || {};
      pdfMake.fonts[DEFAULT_PDF_FONT] = {
        normal: DEJAVU_FONT_FILES.normal,
        bold: DEJAVU_FONT_FILES.bold,
        italics: DEJAVU_FONT_FILES.italics,
        bolditalics: DEJAVU_FONT_FILES.bolditalics
      };
    });
  }

  // Load pdfmake + html-to-pdfmake + DejaVu fonts once, on demand. Subsequent
  // calls reuse the same promise (and thus the browser cache).
  function loadPdfLibraries() {
    if (librariesPromise) {
      return librariesPromise;
    }
    var libsReady =
      typeof pdfMake !== 'undefined' && typeof htmlToPdfmake !== 'undefined'
        ? Promise.resolve()
        : PDF_LIBS.reduce(function (chain, name) {
            return chain.then(function () {
              return loadScript(ASSET_BASE.dir + name + ASSET_BASE.query);
            });
          }, Promise.resolve());

    librariesPromise = libsReady.then(function () {
      return loadDejaVuFonts();
    });
    // Reset on failure so a later click can retry.
    librariesPromise.catch(function () { librariesPromise = null; });
    return librariesPromise;
  }

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
    // YYYY-MM-DD — locale-neutral, unambiguous.
    return (
      d.getFullYear() +
      '-' + pad(d.getMonth() + 1) +
      '-' + pad(d.getDate())
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
    normalizeDetails(clone);
    absolutizeLinks(clone);

    return clone;
  }

  // Details (collapsible sections) use <a href="javascript:void(0)"
  // class="details__summary"> as the clickable title. In the PDF that would
  // render as a blue hyperlink with a useless javascript: target. Unwrap the
  // summary into a <strong> so it stays plain bold text; the surrounding
  // .details box is restyled as a gray callout in customTag.
  function normalizeDetails(root) {
    root.querySelectorAll('a.details__summary').forEach(function (a) {
      var strong = document.createElement('strong');
      strong.className = 'details__summary';
      while (a.firstChild) {
        strong.appendChild(a.firstChild);
      }
      a.parentNode.replaceChild(strong, a);
    });
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

  // Colored-square status emojis (used in comparison/matrix tables) that even
  // DejaVu can't render — they show up as empty "tofu" boxes. They are
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

  // Walk the pdfmake tree and replace unrenderable status-square emojis. DejaVu
  // (and most text fonts) have no glyph for them (they render as tofu), so a
  // text node that is exactly one such emoji is turned into a colored vector
  // square. Any emoji left inside a longer string is stripped so no tofu remains.
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

  // Collapsible .details sections: same callout chrome as alerts, gray fill.
  var DETAILS_COLORS = { fill: '#f2f4f7', bar: '#9aa0a6' };

  // Heading styling. `top`/`bottom` are the margins (in pt) added above / kept
  // below each heading — html-to-pdfmake gives headings only a small marginBottom
  // and no marginTop, so they crowd the text above and float far from the text
  // they introduce. `size` mirrors the site's heading scale (site px * ~0.75 pt),
  // keeping a clear h1 > h2 > h3 hierarchy that pdfmake otherwise flattens.
  // `top` separates a heading from the block above it; `bottom` from the text it
  // introduces. Body text is 10pt at lineHeight 1.3 (~13pt/line), so `top` ≈ one
  // line keeps consecutive headings about a line apart, while a modest `bottom`
  // gives a small but visible gap before the heading's own paragraph.
  var HEADING_STYLE = {
    h1: { top: 16, bottom: 7, size: 27, bold: true },
    h2: { top: 14, bottom: 7, size: 18, bold: true },
    h3: { top: 12, bottom: 6, size: 15, bold: true },
    h4: { top: 12, bottom: 6, size: 13, bold: true },
    h5: { top: 10, bottom: 5, size: 11, bold: true },
    h6: { top: 10, bottom: 4, size: 10, bold: true }
  };

  var CODE_FILL = '#f6f8fa';

  // Wrap every code block in a full-width gray panel. Runs as a tree pass (not
  // via customTag: html-to-pdfmake handles <pre> in its own switch branch that
  // never calls customTag). A PRE node arrives as
  //   { nodeName:'PRE', text:[...runs], fontSize, preserveLeadingSpaces, style }
  // and is replaced by a single-cell table whose fill spans the whole width.
  //
  // CRITICAL: the replacement table must sit inside a `stack` container, never a
  // `text` array — pdfmake silently drops table/block objects placed in `text`,
  // which makes the whole code block vanish. Since PRE nodes are typically found
  // inside their container's `text` array, we box at the CONTAINER level: when a
  // node has a `text`/`stack` array containing a PRE, we box that PRE in place
  // and, if the container used `text`, rename it to `stack` so the table renders.
  function styleCodeBlocks(node) {
    if (!node || typeof node !== 'object') {
      return;
    }
    if (Array.isArray(node)) {
      for (var j = 0; j < node.length; j++) {
        var item = node[j];
        // A PRE sitting directly in a block-level array (e.g. the top-level
        // content array or a `stack`) can be boxed in place — arrays here render
        // as blocks, so a table is valid.
        if (item && typeof item === 'object' && item.nodeName === 'PRE') {
          node[j] = boxCodeNode(item);
        } else {
          styleCodeBlocks(item);
        }
      }
      return;
    }

    ['stack', 'text'].forEach(function (containerKey) {
      var arr = node[containerKey];
      if (!Array.isArray(arr)) {
        return;
      }
      var hasPre = false;
      for (var i = 0; i < arr.length; i++) {
        var child = arr[i];
        if (child && typeof child === 'object' && child.nodeName === 'PRE') {
          arr[i] = boxCodeNode(child);
          hasPre = true;
        }
      }
      // A table can't live in a `text` array — promote it to `stack`.
      if (hasPre && containerKey === 'text') {
        node.stack = arr;
        delete node.text;
      }
    });

    Object.keys(node).forEach(function (k) {
      var value = node[k];
      if (value && typeof value === 'object') {
        styleCodeBlocks(value);
      }
    });
  }

  // A node that is only whitespace text with no block/structural payload.
  function isBlankNode(n) {
    if (!n || typeof n !== 'object' || Array.isArray(n)) {
      return false;
    }
    if (typeof n.text !== 'string' || n.text.trim() !== '') {
      return false;
    }
    // Must not carry any block/structural content — only inert text-styling
    // keys. (Empty inline elements like <i> </i> between blocks arrive with
    // italics/bold/decoration set and must still count as blank.)
    var inert = {
      text: 1, style: 1, fontSize: 1, preserveLeadingSpaces: 1, nodeName: 1,
      italics: 1, bold: 1, decoration: 1, color: 1, background: 1, lineHeight: 1
    };
    return Object.keys(n).every(function (k) { return inert[k]; });
  }

  // Recursively drop blank-only nodes from BLOCK arrays (content / stack), but
  // never from `text` arrays (where a lone space is a real inter-word space).
  // `inTextArray` marks whether the array currently being scanned is a `text`.
  function dropBlankBlockNodes(node, inTextArray) {
    if (!node || typeof node !== 'object') {
      return;
    }
    if (Array.isArray(node)) {
      if (!inTextArray) {
        for (var i = node.length - 1; i >= 0; i--) {
          if (isBlankNode(node[i])) {
            node.splice(i, 1);
          }
        }
      }
      node.forEach(function (child) { dropBlankBlockNodes(child, inTextArray); });
      return;
    }
    Object.keys(node).forEach(function (k) {
      var value = node[k];
      if (value && typeof value === 'object') {
        dropBlankBlockNodes(value, k === 'text');
      }
    });
  }

  // A single inline text run: plain string `text`, no block containers. Used to
  // detect stacks that should have been inline `text` arrays.
  function isInlineTextRun(n) {
    if (!n || typeof n !== 'object' || Array.isArray(n)) {
      return false;
    }
    if (typeof n.text !== 'string') {
      return false;
    }
    return !(n.stack || n.ul || n.ol || n.table || n.columns || n.image || n.canvas);
  }

  // html-to-pdfmake's <li> handler peels nested lists off a stack and wraps the
  // leftover leading content as `{ stack: [run, run, ...] }`. In pdfmake a
  // `stack` is vertical, so "В ModuleConfig" / `deckhouse` / ":" each land on
  // their own line. When every stack child is an inline text run, promote the
  // stack to a `text` array so the runs stay on one line.
  function flattenInlineStacks(node) {
    if (!node || typeof node !== 'object') {
      return;
    }
    if (Array.isArray(node)) {
      node.forEach(flattenInlineStacks);
      return;
    }
    // Depth-first: flatten nested stacks before inspecting this node.
    Object.keys(node).forEach(function (k) {
      if (node[k] && typeof node[k] === 'object') {
        flattenInlineStacks(node[k]);
      }
    });
    if (Array.isArray(node.stack) &&
        node.stack.length > 0 &&
        node.stack.every(isInlineTextRun)) {
      node.text = node.stack;
      delete node.stack;
    }
  }

  // <a><code>…</code></a> arrives as a linked run that also has the code
  // background. html-to-pdfmake then applies link chrome (blue + underline),
  // which overpowers the inline-code look. Keep the link, drop the chrome.
  function softenCodeLinks(node) {
    if (!node || typeof node !== 'object') {
      return;
    }
    if (Array.isArray(node)) {
      node.forEach(softenCodeLinks);
      return;
    }
    if (node.link && node.background) {
      delete node.color;
      delete node.decoration;
    }
    Object.keys(node).forEach(function (k) {
      if (node[k] && typeof node[k] === 'object') {
        softenCodeLinks(node[k]);
      }
    });
  }

  // Top margin (pt) for a heading that immediately follows another heading, so
  // back-to-back headings (e.g. h2 then h3 with no text between) sit close
  // instead of stacking both headings' full vertical margins.
  var ADJACENT_HEADING_TOP = 4;

  function isHeadingNode(n) {
    return n && typeof n === 'object' && n.headlineLevel;
  }

  // In every block array (content / stack), when a heading directly follows
  // another heading, shrink the second one's top margin.
  function tightenAdjacentHeadings(node, inTextArray) {
    if (!node || typeof node !== 'object') {
      return;
    }
    if (Array.isArray(node)) {
      if (!inTextArray) {
        for (var i = 1; i < node.length; i++) {
          if (isHeadingNode(node[i]) && isHeadingNode(node[i - 1])) {
            var m = node[i].margin;
            if (Array.isArray(m)) {
              m[1] = ADJACENT_HEADING_TOP;
            }
          }
        }
      }
      node.forEach(function (child) { tightenAdjacentHeadings(child, inTextArray); });
      return;
    }
    Object.keys(node).forEach(function (k) {
      var value = node[k];
      if (value && typeof value === 'object') {
        tightenAdjacentHeadings(value, k === 'text');
      }
    });
  }

  function boxCodeNode(preNode) {
    // Strip per-run gray fill so it doesn't double up under the panel fill.
    stripBackground(preNode);
    // Drop the PRE marker so the tree walk can't match & re-box it once it's
    // nested inside the cell below.
    delete preNode.nodeName;
    return {
      table: {
        widths: ['*'],
        body: [[{
          stack: [preNode],
          fillColor: CODE_FILL,
          margin: [8, 6, 8, 6]
        }]]
      },
      layout: 'noBorders',
      margin: [0, 6, 0, 8]
    };
  }

  function stripBackground(node) {
    if (!node || typeof node !== 'object') {
      return;
    }
    if (Array.isArray(node)) {
      node.forEach(stripBackground);
      return;
    }
    delete node.background;
    Object.keys(node).forEach(function (k) {
      if (node[k] && typeof node[k] === 'object') {
        stripBackground(node[k]);
      }
    });
  }

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
  //   - .details      -> the same box, with a gray fill (always expanded);
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

    if (tag === 'div' && el.classList && el.classList.contains('details')) {
      return alertBox(ret, DETAILS_COLORS);
    }

    if (tag === 'blockquote') {
      return alertBox(ret, ALERT_COLORS.info);
    }

    // NOTE: <pre> is NOT handled here — html-to-pdfmake has its own `case "PRE"`
    // branch that never reaches customTag. Code blocks are boxed later, in the
    // styleCodeBlocks() tree pass.

    if (tag === 'p') {
      // A paragraph margin must live only on the block node. When a paragraph
      // has inline children (e.g. <code>, <a>), html-to-pdfmake splits it into
      // an array of text runs and applies defaultStyles.p to EACH run — so the
      // per-run vertical margins stack and inflate the paragraph. Set the margin
      // on the block here and strip it from the inner runs.
      if (Array.isArray(ret.text)) {
        ret.text.forEach(function (run) {
          if (run && typeof run === 'object') {
            delete run.margin;
            delete run.marginTop;
            delete run.marginBottom;
          }
        });
      }
      ret.margin = [0, 0, 0, 6];
      return ret;
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
      // The doc's global lineHeight (1.3, tuned for 10pt body text) inflates a
      // heading's own line box far more at these larger sizes — e.g. 18pt * 1.3
      // adds ~5pt of padding baked into the line itself, on top of the margin
      // above. That extra padding is what made the heading-to-paragraph gap look
      // much bigger than the configured margin. Headings render as a single
      // line, so a tight lineHeight removes that inflation without affecting
      // readability.
      ret.lineHeight = 1;
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
      // Code isn't set in a monospace face (Courier isn't in the vfs). Inline
      // <code> gets a light fill here; code blocks get a full-width gray panel
      // in styleCodeBlocks() (customTag can't reach <pre>). Body text uses
      // DejaVu (see defaultStyle.font) for broad Unicode coverage.
      defaultStyles: {
        code: { background: '#f2f4f7' },
        pre: { fontSize: 8.5, preserveLeadingSpaces: true }
        // NOTE: paragraph spacing is handled in customTag (tag === 'p'), not
        // here. defaultStyles.p leaks its margin onto every inline text run of a
        // split paragraph, which stacks vertically and inflates the gap after
        // headings — see the customTag 'p' branch.
      },
      customTag: customTag
    });

    // Definitive guard: whatever width/height html-to-pdfmake derived from the
    // markup, normalize every image node to a valid numeric width computed from
    // the actual PNG bytes. This prevents pdfmake's "unsupported number: NaN"
    // crash in renderImage when a bad/absent dimension slips through.
    normalizeImageNodes(content);

    // Remove blank-only nodes sitting at block level. html-to-pdfmake turns the
    // whitespace/newlines between block tags into stray { text: " " } nodes; in a
    // block context each renders as an empty line, inflating the gap before
    // headings well beyond their configured margin. (Blank runs inside a `text`
    // array are real inter-word spaces and are left alone.)
    dropBlankBlockNodes(content, false);

    // Fix list lead-ins that html-to-pdfmake incorrectly put in a vertical
    // `stack` (e.g. "В ModuleConfig `deckhouse`:") so they stay on one line.
    flattenInlineStacks(content);

    // Prefer inline-code appearance over blue underline for <a><code>.
    softenCodeLinks(content);

    // Tighten the gap between back-to-back headings (e.g. an h2 immediately
    // followed by an h3 with no text between). Must run after blank nodes are
    // dropped so the two headings are actually adjacent in the array.
    tightenAdjacentHeadings(content);

    // Wrap code blocks in a full-width gray panel (html-to-pdfmake leaves <pre>
    // as flat text runs with only per-glyph background).
    styleCodeBlocks(content);

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
        font: DEFAULT_PDF_FONT,
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

    // Fetch the heavy libraries on demand, then inline images (pdfmake cannot
    // resolve image URLs in the browser) and finally generate the PDF.
    loadPdfLibraries()
      .then(function () {
        return inlineImages(contentClone);
      })
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

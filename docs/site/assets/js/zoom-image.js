// document.addEventListener('DOMContentLoaded', function () {
//   const content = document.querySelector('.content');
//   if (!content) return;

//   const images = content.querySelectorAll('img[src]');
//   if (!images.length) return;

//   let overlay = null;
//   let zoomLevel = 1;
//   let currentImg = null;

//   const ZOOM_STEP_CLICK = 0.12;
//   const ZOOM_STEP_WHEEL = 0.08;
//   const ZOOM_STEP_KEYS = 0.15;
//   const ZOOM_MIN = 0.3;
//   const ZOOM_MAX = 3;

//   function openOverlay(src) {
//     if (overlay) return;

//     zoomLevel = 1;
//     overlay = document.createElement('div');
//     overlay.className = 'zoom-image-overlay';
//     overlay.innerHTML = [
//       '<button type="button" class="zoom-image-close" aria-label="Закрыть">&times;</button>',
//       '<div class="zoom-image-wrap">',
//       '  <img class="zoom-image-img" src="" alt="">',
//       '</div>'
//     ].join('');

//     currentImg = overlay.querySelector('.zoom-image-img');
//     currentImg.src = src;
//     currentImg.style.transform = 'scale(0.5)';

//     function close() {
//       if (!overlay) return;
//       overlay.remove();
//       overlay = null;
//       document.removeEventListener('keydown', onKey);
//       document.body.style.overflow = '';
//     }

//     function setZoom(delta) {
//       zoomLevel = Math.max(ZOOM_MIN, Math.min(ZOOM_MAX, zoomLevel + delta));
//       currentImg.style.transform = 'scale(' + zoomLevel + ')';
//     }

//     function onKey(e) {
//       if (e.key === 'Escape') close();
//       if (e.key === '+' || e.key === '=') setZoom(ZOOM_STEP_KEYS);
//       if (e.key === '-') setZoom(-ZOOM_STEP_KEYS);
//     }

//     overlay.querySelector('.zoom-image-close').addEventListener('click', function (e) {
//       e.stopPropagation();
//       close();
//     });

//     overlay.addEventListener('click', function (e) {
//       if (e.target === overlay) close();
//     });

//     const wrap = overlay.querySelector('.zoom-image-wrap');
//     wrap.addEventListener('click', function (e) {
//       e.stopPropagation();
//       if (e.target.classList.contains('zoom-image-img')) {
//         setZoom(ZOOM_STEP_CLICK);
//       } else {
//         close();
//       }
//     });

//     wrap.addEventListener('wheel', function (e) {
//       e.preventDefault();
//       setZoom(e.deltaY > 0 ? -ZOOM_STEP_WHEEL : ZOOM_STEP_WHEEL);
//     }, { passive: false });

//     document.addEventListener('keydown', onKey);
//     document.body.style.overflow = 'hidden';
//     document.body.appendChild(overlay);
//   }

//   images.forEach(function (img) {
//     const w = img.getAttribute('width') || img.offsetWidth;
//     const h = img.getAttribute('height') || img.offsetHeight;
//     if (w && parseInt(w, 10) < 40 && h && parseInt(h, 10) < 40) return;

//     const src = img.getAttribute('src');
//     if (!src) return;

//     img.style.cursor = 'pointer';
//     img.addEventListener('click', function (e) {
//       e.preventDefault();
//       openOverlay(img.currentSrc || img.src);
//     });
//   });
// });

// Lightbox через GLightbox, но зум/пан/кнопки управляются нашим кодом
document.addEventListener('DOMContentLoaded', function () {
  const content = document.querySelector('.content');
  if (!content || typeof GLightbox === 'undefined') return;

  const ZOOM_STEP = 0.25;
  const ZOOM_MIN = 1;
  const ZOOM_MAX = 4;

  function waitForLightbox(cb) {
    let tries = 0;
    function check() {
      const container = document.querySelector('.glightbox-container');
      if (container) return cb(container);
      tries += 1;
      if (tries < 60) setTimeout(check, 50);
    }
    setTimeout(check, 50);
  }

  function getActiveImage(container) {
    return container.querySelector('.gslide.current img, .gslide-current img, .gslide img');
  }

  function getState(container) {
    if (!container.__zoomImageState) {
      container.__zoomImageState = { scale: 1, x: 0, y: 0, dragging: false };
    }
    return container.__zoomImageState;
  }

  function applyTransform(container) {
    const img = getActiveImage(container);
    if (!img) return;
    const state = getState(container);
    img.style.transform = 'translate(' + state.x + 'px,' + state.y + 'px) scale(' + state.scale + ')';
    img.style.transformOrigin = 'center center';
    img.style.transition = state.dragging ? 'none' : 'transform 0.12s ease';
    img.style.willChange = 'transform';
    if (state.scale > 1) {
      img.style.cursor = state.dragging ? 'grabbing' : 'grab';
    } else {
      img.style.cursor = 'pointer';
    }
    img.style.userSelect = 'none';
    img.style.touchAction = 'none';
    img.draggable = false;
  }

  function setScale(container, nextScale) {
    const state = getState(container);
    state.scale = Math.max(ZOOM_MIN, Math.min(ZOOM_MAX, nextScale));
    if (state.scale <= 1) {
      state.x = 0;
      state.y = 0;
    }
    applyTransform(container);
  }

  function addToolbar(container) {
    const closeBtn = container.querySelector('.gclose');
    if (!closeBtn) return;
    const oldToolbar = document.querySelector('.zoom-image-toolbar');
    if (oldToolbar) oldToolbar.remove();

    const toolbar = document.createElement('div');
    toolbar.className = 'zoom-image-toolbar';
    toolbar.style.position = 'fixed';
    toolbar.style.top = '12px';
    toolbar.style.right = '12px';
    toolbar.style.zIndex = '2147483647';
    toolbar.style.display = 'flex';
    toolbar.style.alignItems = 'center';
    toolbar.style.gap = '8px';
    toolbar.style.pointerEvents = 'auto';

    const zoomOut = document.createElement('button');
    zoomOut.type = 'button';
    zoomOut.className = 'zoom-image-zoom-btn zoom-image-zoom-out';
    zoomOut.setAttribute('aria-label', 'Отдалить');
    zoomOut.innerHTML = '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/><path d="M8 11h6"/></svg>';
    zoomOut.style.width = '40px';
    zoomOut.style.height = '40px';
    zoomOut.style.border = '0';
    zoomOut.style.borderRadius = '4px';
    zoomOut.style.background = 'rgba(255,255,255,0.2)';
    zoomOut.style.color = '#fff';
    zoomOut.style.display = 'flex';
    zoomOut.style.alignItems = 'center';
    zoomOut.style.justifyContent = 'center';
    zoomOut.style.cursor = 'pointer';
    zoomOut.style.padding = '0';

    const zoomIn = document.createElement('button');
    zoomIn.type = 'button';
    zoomIn.className = 'zoom-image-zoom-btn zoom-image-zoom-in';
    zoomIn.setAttribute('aria-label', 'Приблизить');
    zoomIn.innerHTML = '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/><path d="M11 8v6"/><path d="M8 11h6"/></svg>';
    zoomIn.style.width = '40px';
    zoomIn.style.height = '40px';
    zoomIn.style.border = '0';
    zoomIn.style.borderRadius = '4px';
    zoomIn.style.background = 'rgba(255,255,255,0.2)';
    zoomIn.style.color = '#fff';
    zoomIn.style.display = 'flex';
    zoomIn.style.alignItems = 'center';
    zoomIn.style.justifyContent = 'center';
    zoomIn.style.cursor = 'pointer';
    zoomIn.style.padding = '0';

    zoomOut.addEventListener('click', function (e) {
      e.preventDefault();
      e.stopPropagation();
      setScale(container, getState(container).scale - ZOOM_STEP);
    });

    zoomIn.addEventListener('click', function (e) {
      e.preventDefault();
      e.stopPropagation();
      setScale(container, getState(container).scale + ZOOM_STEP);
    });

    const closeProxy = document.createElement('button');
    closeProxy.type = 'button';
    closeProxy.className = 'zoom-image-zoom-btn zoom-image-close-proxy';
    closeProxy.setAttribute('aria-label', 'Закрыть');
    closeProxy.innerHTML = '&times;';
    closeProxy.style.width = '40px';
    closeProxy.style.height = '40px';
    closeProxy.style.border = '0';
    closeProxy.style.borderRadius = '4px';
    closeProxy.style.background = 'rgba(255,255,255,0.2)';
    closeProxy.style.color = '#fff';
    closeProxy.style.fontSize = '28px';
    closeProxy.style.lineHeight = '1';
    closeProxy.style.display = 'flex';
    closeProxy.style.alignItems = 'center';
    closeProxy.style.justifyContent = 'center';
    closeProxy.style.cursor = 'pointer';
    closeProxy.style.padding = '0';
    closeProxy.addEventListener('click', function (e) {
      e.preventDefault();
      e.stopPropagation();
      document.body.style.cursor = '';
      closeBtn.click();
      if (toolbar.parentNode) toolbar.parentNode.removeChild(toolbar);
    });

    toolbar.appendChild(zoomOut);
    toolbar.appendChild(zoomIn);
    toolbar.appendChild(closeProxy);
    document.body.appendChild(toolbar);

    // Скрываем родной крестик GLightbox, чтобы не дублировался.
    closeBtn.style.display = 'none';

    // Чистим тулбар, когда lightbox закрыт.
    let cleanupTries = 0;
    function cleanupToolbarWhenClosed() {
      cleanupTries += 1;
      if (!document.body.contains(container)) {
        document.body.style.cursor = '';
        if (toolbar.parentNode) toolbar.parentNode.removeChild(toolbar);
        return;
      }
      if (cleanupTries < 400) setTimeout(cleanupToolbarWhenClosed, 50);
    }
    setTimeout(cleanupToolbarWhenClosed, 50);
  }

  function enablePanAndWheelZoom(container) {
    const img = getActiveImage(container);
    if (!img || img.dataset.zoomImageBound === '1') return;
    img.dataset.zoomImageBound = '1';

    const state = getState(container);
    let startX = 0;
    let startY = 0;
    let baseX = 0;
    let baseY = 0;

    function pointerDown(e) {
      if (e.type === 'mousedown' && e.button !== 0) return;
      if (state.scale <= 1) return;
      const point = e.touches && e.touches[0] ? e.touches[0] : e;
      state.dragging = true;
      document.body.style.cursor = 'grabbing';
      startX = point.clientX;
      startY = point.clientY;
      baseX = state.x;
      baseY = state.y;
      applyTransform(container);
      e.preventDefault();
      e.stopPropagation();
      document.addEventListener('mousemove', pointerMove, true);
      document.addEventListener('mouseup', pointerUp, true);
      document.addEventListener('touchmove', pointerMove, { passive: false, capture: true });
      document.addEventListener('touchend', pointerUp, true);
    }

    function pointerMove(e) {
      if (!state.dragging) return;
      const point = e.touches && e.touches[0] ? e.touches[0] : e;
      const dx = point.clientX - startX;
      const dy = point.clientY - startY;
      state.x = baseX + dx;
      state.y = baseY + dy;
      applyTransform(container);
      e.preventDefault();
      e.stopPropagation();
    }

    function pointerUp() {
      if (!state.dragging) return;
      state.dragging = false;
      document.body.style.cursor = '';
      applyTransform(container);
      document.removeEventListener('mousemove', pointerMove, true);
      document.removeEventListener('mouseup', pointerUp, true);
      document.removeEventListener('touchmove', pointerMove, true);
      document.removeEventListener('touchend', pointerUp, true);
    }

    function wheelZoom(e) {
      e.preventDefault();
      e.stopPropagation();
      const delta = e.deltaY < 0 ? ZOOM_STEP : -ZOOM_STEP;
      setScale(container, state.scale + delta);
    }

    img.addEventListener('mousedown', pointerDown, true);
    img.addEventListener('touchstart', pointerDown, { passive: false, capture: true });
    img.addEventListener('wheel', wheelZoom, { passive: false });

    applyTransform(container);
  }

  const images = content.querySelectorAll('img[src]');
  if (!images.length) return;

  images.forEach(function (img) {
    const w = img.getAttribute('width') || img.offsetWidth;
    const h = img.getAttribute('height') || img.offsetHeight;
    if (w && parseInt(w, 10) < 40 && h && parseInt(h, 10) < 40) return;

    const src = img.getAttribute('src');
    if (!src) return;

    img.style.cursor = 'pointer';
    img.addEventListener('click', function (e) {
      e.preventDefault();
      const url = img.currentSrc || img.src;
      const lb = GLightbox({
        elements: [{ href: url, type: 'image' }],
        touchNavigation: false,
        loop: false,
        zoomable: false,
        draggable: false,
        closeButton: true,
        openEffect: 'zoom',
        closeEffect: 'fade'
      });
      lb.open();
      waitForLightbox(function (container) {
        let tries = 0;
        function initWhenReady() {
          const imgInSlide = getActiveImage(container);
          const closeInSlide = container.querySelector('.gclose');
          if (imgInSlide && closeInSlide) {
            addToolbar(container);
            enablePanAndWheelZoom(container);
            applyTransform(container);
            return;
          }
          tries += 1;
          if (tries < 80) setTimeout(initWhenReady, 50);
        }
        initWhenReady();
      });
    });
  });
});

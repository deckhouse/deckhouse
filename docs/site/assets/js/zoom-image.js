document.addEventListener('DOMContentLoaded', function () {
  const content = document.querySelector('.content');
  if (!content || typeof GLightbox === 'undefined') return;

  const ZOOM_STEP = 0.25;
  const ZOOM_MIN = 1;
  const ZOOM_MAX = 4;

  function stopEvent(e) {
    e.preventDefault();
    e.stopPropagation();
  }

  function pollUntilReady(getValue, onReady, maxTries) {
    let tries = 0;
    function check() {
      const value = getValue();
      if (value) {
        onReady(value);
        return;
      }
      tries += 1;
      if (tries < maxTries) setTimeout(check, 50);
    }
    setTimeout(check, 50);
  }

  function waitForLightbox(cb) {
    pollUntilReady(function () {
      return document.querySelector('.glightbox-container');
    }, cb, 60);
  }

  function isSvgUrl(url) {
    return /\.svg(?:[?#]|$)/i.test(url || '');
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
    img.style.transformOrigin = 'center center';
    img.style.transform = 'translate(' + state.x + 'px,' + state.y + 'px) scale(' + state.scale + ')';
    img.style.transition = state.dragging ? 'none' : 'transform 0.12s ease';
    if (state.scale > 1) {
      img.style.cursor = state.dragging ? 'grabbing' : 'grab';
    } else {
      img.style.cursor = 'pointer';
    }
    img.draggable = false;
  }

  function clampPan(container) {
    const img = getActiveImage(container);
    if (!img) return;
    const state = getState(container);
    if (state.scale <= 1) {
      state.x = 0;
      state.y = 0;
      return;
    }
    const maxX = Math.max(0, ((img.clientWidth * state.scale) - img.clientWidth) / 2);
    const maxY = Math.max(0, ((img.clientHeight * state.scale) - img.clientHeight) / 2);
    state.x = Math.min(maxX, Math.max(-maxX, state.x));
    state.y = Math.min(maxY, Math.max(-maxY, state.y));
  }

  function setScale(container, nextScale, focusPoint) {
    const img = getActiveImage(container);
    if (!img) return;
    const state = getState(container);
    const prevScale = state.scale;
    const clampedScale = Math.max(ZOOM_MIN, Math.min(ZOOM_MAX, nextScale));
    if (clampedScale === prevScale) return;
    if (clampedScale > 1 && focusPoint && prevScale > 0) {
      const rect = img.getBoundingClientRect();
      const centerX = rect.left + (rect.width / 2);
      const centerY = rect.top + (rect.height / 2);
      const offsetX = focusPoint.x - centerX;
      const offsetY = focusPoint.y - centerY;
      state.x += ((prevScale - clampedScale) / prevScale) * offsetX;
      state.y += ((prevScale - clampedScale) / prevScale) * offsetY;
    }
    state.scale = clampedScale;
    if (state.scale <= 1) {
      state.x = 0;
      state.y = 0;
    }
    clampPan(container);
    applyTransform(container);
  }

  function fitSvgToViewport(container, sourceImg, url) {
    if (!isSvgUrl(url)) return;
    const imgInSlide = getActiveImage(container);
    if (!imgInSlide) return;

    const sourceWidth = sourceImg && sourceImg.clientWidth ? sourceImg.clientWidth : sourceImg.naturalWidth;
    const sourceHeight = sourceImg && sourceImg.clientHeight ? sourceImg.clientHeight : sourceImg.naturalHeight;
    const ratio = sourceWidth > 0 && sourceHeight > 0 ? (sourceWidth / sourceHeight) : (4 / 3);
    const maxWidth = Math.max(320, Math.floor(window.innerWidth * 0.92));
    const maxHeight = Math.max(240, Math.floor(window.innerHeight * 0.86));

    let targetWidth = maxWidth;
    let targetHeight = Math.round(targetWidth / ratio);
    if (targetHeight > maxHeight) {
      targetHeight = maxHeight;
      targetWidth = Math.round(targetHeight * ratio);
    }

    imgInSlide.classList.add('zoom-image-svg');
    imgInSlide.style.width = targetWidth + 'px';
    imgInSlide.style.height = targetHeight + 'px';
  }

  function addToolbar(container) {
    const closeBtn = container.querySelector('.gclose');
    if (!closeBtn) return;
    const oldToolbar = document.querySelector('.zoom-image-toolbar');
    if (oldToolbar) oldToolbar.remove();

    const toolbar = document.createElement('div');
    toolbar.className = 'zoom-image-toolbar';

    function createIconButton(className, label, iconSrc) {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = className;
      btn.setAttribute('aria-label', label);
      if (iconSrc) {
        const icon = document.createElement('img');
        icon.src = iconSrc;
        icon.alt = '';
        icon.width = 22;
        icon.height = 22;
        icon.decoding = 'async';
        btn.appendChild(icon);
      }
      return btn;
    }

    const zoomOut = createIconButton('zoom-image-zoom-btn zoom-image-zoom-out', 'Отдалить', '/images/zoom-out.svg');
    const zoomIn = createIconButton('zoom-image-zoom-btn zoom-image-zoom-in', 'Приблизить', '/images/zoom-in.svg');

    zoomOut.addEventListener('click', function (e) {
      stopEvent(e);
      setScale(container, getState(container).scale - ZOOM_STEP);
    });

    zoomIn.addEventListener('click', function (e) {
      stopEvent(e);
      setScale(container, getState(container).scale + ZOOM_STEP);
    });

    const closeProxy = createIconButton('zoom-image-zoom-btn zoom-image-close-proxy', 'Закрыть');
    closeProxy.innerHTML = '&times;';
    closeProxy.style.fontSize = '28px';
    closeProxy.style.lineHeight = '1';
    closeProxy.addEventListener('click', function (e) {
      stopEvent(e);
      document.body.style.cursor = '';
      closeBtn.click();
      if (toolbar.parentNode) toolbar.parentNode.removeChild(toolbar);
    });

    toolbar.appendChild(zoomOut);
    toolbar.appendChild(zoomIn);
    toolbar.appendChild(closeProxy);
    document.body.appendChild(toolbar);

    closeBtn.style.display = 'none';

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
    let pointerId = null;
    let startX = 0;
    let startY = 0;
    let baseX = 0;
    let baseY = 0;
    let moved = false;
    let rafId = 0;

    function applyTransformRaf() {
      if (rafId) return;
      rafId = window.requestAnimationFrame(function () {
        rafId = 0;
        applyTransform(container);
      });
    }

    function pointerDown(e) {
      if (e.button !== 0 || state.scale <= 1 || pointerId !== null) return;
      pointerId = e.pointerId;
      moved = false;
      startX = e.clientX;
      startY = e.clientY;
      baseX = state.x;
      baseY = state.y;
      img.setPointerCapture(pointerId);
      stopEvent(e);
    }

    function pointerMove(e) {
      if (pointerId === null || e.pointerId !== pointerId) return;
      const dx = e.clientX - startX;
      const dy = e.clientY - startY;
      if (!moved && Math.hypot(dx, dy) < 4) return;
      moved = true;
      if (!state.dragging) {
        state.dragging = true;
        document.body.style.cursor = 'grabbing';
      }
      state.x = baseX + dx;
      state.y = baseY + dy;
      clampPan(container);
      applyTransformRaf();
      stopEvent(e);
    }

    function pointerUp(e) {
      if (pointerId === null || e.pointerId !== pointerId) return;
      if (img.hasPointerCapture(pointerId)) img.releasePointerCapture(pointerId);
      pointerId = null;
      if (!state.dragging) return;
      state.dragging = false;
      document.body.style.cursor = '';
      applyTransform(container);
    }

    function wheelZoom(e) {
      stopEvent(e);
      const delta = e.deltaY < 0 ? ZOOM_STEP : -ZOOM_STEP;
      setScale(container, state.scale + delta, { x: e.clientX, y: e.clientY });
    }

    function clickZoom(e) {
      if (state.dragging || moved) {
        stopEvent(e);
        return;
      }
      stopEvent(e);
      setScale(container, state.scale + ZOOM_STEP, {
        x: window.innerWidth / 2,
        y: window.innerHeight / 2
      });
    }

    img.addEventListener('pointerdown', pointerDown, true);
    img.addEventListener('pointermove', pointerMove, true);
    img.addEventListener('pointerup', pointerUp, true);
    img.addEventListener('pointercancel', pointerUp, true);
    img.addEventListener('click', clickZoom, true);
    img.addEventListener('wheel', wheelZoom, { passive: false });

    applyTransform(container);
  }

  const images = content.querySelectorAll('img[src]');
  if (!images.length) return;

  images.forEach(function (img) {
    if (img.closest('.oss__item-logo')) return;
    if (img.classList.contains('zoom-image-disable')) return;

    const w = img.getAttribute('width') || img.offsetWidth;
    const h = img.getAttribute('height') || img.offsetHeight;
    if (w && parseInt(w, 10) < 40 && h && parseInt(h, 10) < 40) return;

    const src = img.getAttribute('src');
    if (!src) return;

    img.style.cursor = 'zoom-in';
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
        pollUntilReady(function () {
          const imgInSlide = getActiveImage(container);
          const closeInSlide = container.querySelector('.gclose');
          return imgInSlide && closeInSlide;
        }, function () {
          fitSvgToViewport(container, img, url);
          addToolbar(container);
          enablePanAndWheelZoom(container);
          applyTransform(container);
        }, 80);
      });
    });

    const parent = img.parentElement;
    if (!parent) return;
    
    if (parent.classList.contains('zoom-image-wrap')) return;

    const wrapper = document.createElement('div');
    wrapper.className = 'zoom-image-wrap';
    parent.insertBefore(wrapper, img);
    wrapper.appendChild(img);
  });
});

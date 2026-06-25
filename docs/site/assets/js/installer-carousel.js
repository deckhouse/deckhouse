(function () {
  'use strict';

  function initCarousel(root) {
    if (root._installerCarouselInitDone) {
      return;
    }
    var viewport = root.querySelector('.installer-carousel__viewport');
    var track = root.querySelector('.installer-carousel__track');
    var slides = root.querySelectorAll('.installer-carousel__slide');
    var prev = root.querySelector('.installer-carousel__prev');
    var next = root.querySelector('.installer-carousel__next');
    var paginationEl = root.querySelector('.installer-carousel__pagination');
    if (!viewport || !track || slides.length === 0) {
      return;
    }

    root._installerCarouselInitDone = true;

    var index = 0;
    var slideCount = slides.length;

    function getPaginationItems(activeIndex, total) {
      var cur = activeIndex + 1;
      if (total <= 1) {
        return [];
      }
      if (total <= 4) {
        var all = [];
        for (var n = 1; n <= total; n++) {
          all.push(n);
        }
        return all;
      }

      var windowStart;
      var windowEnd;

      if (cur <= 3) {
        windowStart = 1;
        windowEnd = Math.min(3, total);
      } else if (cur >= total - 2) {
        windowStart = total - 2;
        windowEnd = total;
      } else {
        windowStart = cur - 2;
        windowEnd = cur;
      }

      var items = [];
      if (cur >= total - 2 && windowStart > 1) {
        items.push(1);
        items.push('ellipsis');
      }
      for (var p = windowStart; p <= windowEnd; p++) {
        items.push(p);
      }
      if (windowEnd < total - 1) {
        items.push('ellipsis');
        items.push(total);
      } else if (windowEnd === total - 1) {
        items.push(total);
      }
      return items;
    }

    function syncNav() {
      if (!prev || !next) {
        return;
      }
      if (slideCount <= 1) {
        prev.setAttribute('hidden', 'hidden');
        next.setAttribute('hidden', 'hidden');
        return;
      }
      prev.removeAttribute('hidden');
      next.removeAttribute('hidden');
      prev.disabled = index <= 0;
      next.disabled = index >= slideCount - 1;
    }

    function updatePagination() {
      if (!paginationEl) {
        return;
      }
      var items = getPaginationItems(index, slideCount);
      paginationEl.innerHTML = '';

      for (var i = 0; i < items.length; i++) {
        var item = items[i];
        if (item === 'ellipsis') {
          var ellipsis = document.createElement('span');
          ellipsis.className = 'installer-carousel__pagination--ellipsis';
          ellipsis.setAttribute('aria-hidden', 'true');
          ellipsis.textContent = '...';
          paginationEl.appendChild(ellipsis);
          continue;
        }

        var pageBtn = document.createElement('button');
        pageBtn.type = 'button';
        pageBtn.className = 'installer-carousel__pagination--page';
        pageBtn.textContent = String(item);
        pageBtn.setAttribute('data-page', String(item));
        if (item === index + 1) {
          pageBtn.classList.add('is-active');
          pageBtn.setAttribute('aria-current', 'step');
        }
        paginationEl.appendChild(pageBtn);
      }
    }

    function goTo(i) {
      index = Math.max(0, Math.min(slideCount - 1, i));
      var w = viewport.clientWidth;
      if (w <= 0) {
        return;
      }
      track.style.transform = 'translateX(' + (-index * w) + 'px)';
      syncNav();
      updatePagination();
    }

    function refresh() {
      var w = viewport.clientWidth;
      if (w <= 0) {
        return;
      }
      for (var s = 0; s < slides.length; s++) {
        slides[s].style.flex = '0 0 ' + w + 'px';
        slides[s].style.width = w + 'px';
        slides[s].style.maxWidth = w + 'px';
      }
      track.style.width = w * slideCount + 'px';
      goTo(index);
    }

    if (prev) {
      prev.addEventListener('click', function () {
        goTo(index - 1);
      });
    }
    if (next) {
      next.addEventListener('click', function () {
        goTo(index + 1);
      });
    }
    if (paginationEl) {
      paginationEl.addEventListener('click', function (e) {
        var btn = e.target.closest('.installer-carousel__pagination--page');
        if (!btn || btn.classList.contains('is-active')) {
          return;
        }
        var page = parseInt(btn.getAttribute('data-page'), 10);
        if (!isNaN(page)) {
          goTo(page - 1);
        }
      });
    }

    var swipeStartX = 0;
    viewport.addEventListener(
      'pointerdown',
      function (e) {
        swipeStartX = e.clientX;
      },
      { passive: true }
    );
    viewport.addEventListener('pointerup', function (e) {
      var dx = e.clientX - swipeStartX;
      if (Math.abs(dx) < 48) {
        return;
      }
      if (dx < 0) {
        goTo(index + 1);
      } else {
        goTo(index - 1);
      }
    });

    window.addEventListener('resize', refresh);

    root._installerCarouselRefresh = refresh;
    refresh();
    syncNav();
  }

  function initAll() {
    document.querySelectorAll('[data-installer-carousel]').forEach(function (root) {
      initCarousel(root);
    });
  }

  window.refreshInstallerCarousels = function () {
    document.querySelectorAll('[data-installer-carousel]').forEach(function (root) {
      if (typeof root._installerCarouselRefresh === 'function') {
        root._installerCarouselRefresh();
      } else {
        initCarousel(root);
      }
    });
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initAll);
  } else {
    initAll();
  }
})();

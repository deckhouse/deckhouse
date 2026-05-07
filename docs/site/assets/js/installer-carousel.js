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
    var counterEl = root.querySelector('.installer-carousel__counter');
    if (!viewport || !track || slides.length === 0) {
      return;
    }

    root._installerCarouselInitDone = true;

    var index = 0;
    var slideCount = slides.length;

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

    function updateCounter() {
      if (!counterEl) {
        return;
      }
      var lang = root.getAttribute('data-counter-lang') || 'ru';
      var cur = index + 1;
      if (lang === 'ru') {
        counterEl.textContent = '(' + cur + ' из ' + slideCount + ')';
      } else {
        counterEl.textContent = '(' + cur + ' of ' + slideCount + ')';
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
      updateCounter();
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

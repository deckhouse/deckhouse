function getPaginationItems(activeIndex, total) {
  if (total <= 1) return [];
  if (total <= 4) {
    const all = [];
    for (let n = 1; n <= total; n++) {
      all.push(n);
    }
    return all;
  }

  const cur = activeIndex + 1;
  const start = cur <= 3 ? 1 : cur >= total - 2 ? total - 2 : cur - 2;
  const end = cur <= 3 ? Math.min(3, total) : cur >= total - 2 ? total : cur;
  const items = [];

  if (cur >= total - 2 && start > 1) items.push(1, 'ellipsis');
  for (let p = start; p <= end; p++) {
    items.push(p);
  }

  if (end < total - 1) items.push('ellipsis', total);
  else if (end === total - 1) items.push(total);

  return items;
}

function createPaginationNode(item, activePage) {
  if (item === 'ellipsis') {
    const node = document.createElement('span');
    node.className = 'slider__pagination--ellipsis';
    node.setAttribute('aria-hidden', 'true');
    node.textContent = '...';
    return node;
  }

  const btn = document.createElement('button');
  btn.type = 'button';
  btn.className = 'slider__pagination--page';
  btn.textContent = String(item);
  btn.dataset.page = String(item);

  if (item === activePage) {
    btn.classList.add('is-active');
    btn.setAttribute('aria-current', 'step');
  }

  return btn;
}

function initCarousel(root) {
  if (root._installerCarouselRefresh) return;

  const viewport = root.querySelector('.slider__viewport');
  const track = root.querySelector('.slider__track');
  const slides = root.querySelectorAll('.slider__slide');
  const prev = root.querySelector('.slider__prev');
  const next = root.querySelector('.slider__next');
  const paginationEl = root.querySelector('.slider__pagination');

  if (!viewport || !track || !slides.length) return;

  let index = 0;
  const slideCount = slides.length;

  function syncNav() {
    if (!prev || !next) return;

    const singleSlide = slideCount <= 1;
    prev.toggleAttribute('hidden', singleSlide);
    next.toggleAttribute('hidden', singleSlide);

    if (!singleSlide) {
      prev.disabled = index <= 0;
      next.disabled = index >= slideCount - 1;
    }
  }

  function updatePagination() {
    if (!paginationEl) return;

    paginationEl.replaceChildren(
      ...getPaginationItems(index, slideCount).map(function (item) {
        return createPaginationNode(item, index + 1);
      })
    );
  }

  function update() {
    const width = viewport.clientWidth;
    if (width <= 0) return;

    track.style.transform = `translateX(${-index * width}px)`;
    syncNav();
    updatePagination();
  }

  function goTo(nextIndex) {
    index = Math.max(0, Math.min(slideCount - 1, nextIndex));
    update();
  }

  function refresh() {
    update();
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
      const btn = e.target.closest('.slider__pagination--page');
      if (!btn || btn.classList.contains('is-active')) return;

      const page = Number(btn.dataset.page);
      if (!Number.isNaN(page)) goTo(page - 1);
    });
  }

  let swipeStartX = 0;

  viewport.addEventListener('pointerdown', function (e) {
    swipeStartX = e.clientX;
  }, { passive: true });

  viewport.addEventListener('pointerup', function (e) {
    const dx = e.clientX - swipeStartX;
    if (Math.abs(dx) >= 48) goTo(dx < 0 ? index + 1 : index - 1);
  });

  new ResizeObserver(refresh).observe(viewport);

  root._installerCarouselRefresh = refresh;
  refresh();
}

function initAll() {
  document.querySelectorAll('[data-slider]').forEach(initCarousel);
}

window.refreshInstallerCarousels = function () {
  document.querySelectorAll('[data-slider]').forEach(function (root) {
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

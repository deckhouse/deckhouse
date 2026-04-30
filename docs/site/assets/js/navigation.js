document.addEventListener('DOMContentLoaded', () => {
    const navigationContainer = document.querySelector('.navigation__container');

    if (!navigationContainer && sidebarAndToc.length === 0) {
        return;
    }

    const sidebarAndToc = document.querySelectorAll('.sidebar__wrapper-inner');
    const header = document.querySelector('header');
    let lastScroll = window.scrollY;
    let rafPending = false;

    // navigationContainer.offsetHeight is stable (doesn't depend on page content),
    // but reading it forces a full reflow on large DOMs. We prime the cache on the
    // 'load' event (after first paint) and use 0 as a safe fallback until then.
    let cachedNavigationHeight = 0;
    window.addEventListener('load', () => {
        cachedNavigationHeight = navigationContainer.offsetHeight;
        const newTop = updateTop();
        scrollHandler(newTop);
    }, { once: true });

    function applyHeaderOffsets() {
        // Read all layout properties first, before any writes, to avoid forced reflow.
        const headerHeight = header ? header.getBoundingClientRect().height : 0;
        // Write phase: apply styles after all reads are done.
        navigationContainer.style.top = `${headerHeight}px`;
        sidebarAndToc.forEach(e => {
            e.style.top = `${headerHeight}px`;
        });
        return { headerHeight, navigationHeight: cachedNavigationHeight };
    }

    function updateTop() {
        const { headerHeight, navigationHeight } = applyHeaderOffsets();
        return headerHeight + navigationHeight;
    }

    let isScroll = true;

    function hideNavigation() {
        if (navigationContainer) {
            navigationContainer.classList.add('hidden');
        }
        lastScroll = window.scrollY;
        sidebarAndToc.forEach(e => {
            e.classList.remove('top');
            e.style.removeProperty('--scroll-top');
        });
    }

    function hideNavigationOnAnchor() {
        const hash = decodeURIComponent(window.location.hash.replace('#', ''));
        if (!hash) {
            return;
        }

        if (!document.getElementById(hash)) {
            return;
        }

        isScroll = false;
        hideNavigation();
        setTimeout(() => {
            isScroll = true;
        }, 500);
    }

    function scrollHandler(newTopValue) {
        if (!isScroll) return;

        const scrollWindowTop = window.scrollY;
        if (scrollWindowTop > lastScroll) {
            if (navigationContainer) {
                navigationContainer.classList.add('hidden');
            }
            lastScroll = scrollWindowTop;
            sidebarAndToc.forEach(e => {
                e.classList.remove('top');
                e.style.removeProperty('--scroll-top');
            });
        } else {
            if (navigationContainer) {
                navigationContainer.classList.remove('hidden');
            }
            lastScroll = scrollWindowTop;
            sidebarAndToc.forEach(e => {
                e.style.setProperty('--scroll-top', `${newTopValue}px`);
                e.classList.add('top');
            });
        }
    }

    document.addEventListener('click', (event) => {
        const target = event.target;
        if (!(target instanceof Element)) {
            return;
        }

        const tocLink = target.closest('.toc-sidebar__item-link');
        const anchorLink = target.closest('.anchorjs-link');
        const faqHeading = target.closest('.docs.faq__container h3');

        if (tocLink || anchorLink || faqHeading) {
            isScroll = false;
            hideNavigation();
            setTimeout(() => {
                isScroll = true;
            }, 500);
        }
    });

    const initTop = updateTop();
    scrollHandler(initTop);

    window.addEventListener('scroll', () => {
        if (rafPending) return;
        rafPending = true;
        requestAnimationFrame(() => {
            rafPending = false;
            const newTop = updateTop();
            scrollHandler(newTop);
        });
    });

    window.addEventListener('resize', () => {
        cachedNavigationHeight = navigationContainer.offsetHeight;
        const newTop = updateTop();
        scrollHandler(newTop)
    });

    window.addEventListener('hashchange', hideNavigationOnAnchor);
    window.addEventListener('load', hideNavigationOnAnchor);

    if (window.scrollY > 0) {
        hideNavigation();
    }

    hideNavigationOnAnchor();
});

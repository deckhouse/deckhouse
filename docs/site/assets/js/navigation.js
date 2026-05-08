document.addEventListener('DOMContentLoaded', () => {
    const navigationContainer = document.querySelector('.navigation__container');
    const sidebarAndToc = document.querySelectorAll('.sidebar__wrapper-inner');

    if (!navigationContainer && sidebarAndToc.length === 0) {
        return;
    }

    let lastScroll = window.scrollY;

    function applyHeaderOffsets() {
        const header = document.querySelector('header');
        const headerHeight = header.getBoundingClientRect().height;
        if (navigationContainer) {
            navigationContainer.style.top = `${headerHeight}px`;
        }
        sidebarAndToc.forEach(e => {
            e.style.top = `${headerHeight}px`;
        });
        return headerHeight;
    }

    function updateTop() {
        const headerHeight = applyHeaderOffsets();
        const navigationHeight = navigationContainer ? navigationContainer.offsetHeight : 0;
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
        if (!tocLink && !anchorLink) {
            return;
        }

        isScroll = false;
        hideNavigation();
        setTimeout(() => {
            isScroll = true;
        }, 500);
    });

    const initTop = updateTop();
    scrollHandler(initTop);

    window.addEventListener('scroll', () => {
        const newTop = updateTop();
        scrollHandler(newTop)
    });

    window.addEventListener('resize', () => {
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

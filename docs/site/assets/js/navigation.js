document.addEventListener('DOMContentLoaded', () => {
    const navigationContainer = document.querySelector('.navigation__container');

    if (!navigationContainer) {
        return;
    }

    const sidebarAndToc = document.querySelectorAll('.sidebar__wrapper-inner');
    let lastScroll = window.scrollY;

    function applyHeaderOffsets() {
        const header = document.querySelector('header');
        const headerHeight = header.offsetHeight;
        navigationContainer.style.top = `${headerHeight}px`;
        sidebarAndToc.forEach(e => {
            e.style.top = `${headerHeight}px`;
        });
        return headerHeight;
    }

    function updateTop() {
        const headerHeight = applyHeaderOffsets();
        const navigationHeight = navigationContainer.offsetHeight;
        return headerHeight + navigationHeight;
    }

    let isScroll = true;

    function hideNavigation() {
        navigationContainer.classList.add('hidden');
        lastScroll = window.scrollY;
        sidebarAndToc.forEach(e => {
            e.classList.remove('top');
            e.style.removeProperty('--scroll-top');
        });
    }

    function scrollHandler(newTopValue) {
        if (!isScroll) return;

        const scrollWindowTop = window.scrollY;
        if (scrollWindowTop > lastScroll) {
            navigationContainer.classList.add('hidden');
            lastScroll = scrollWindowTop;
            sidebarAndToc.forEach(e => {
                e.classList.remove('top');
                e.style.removeProperty('--scroll-top');
            });
        } else {
            navigationContainer.classList.remove('hidden');
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
        if (!tocLink) {
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

    if (window.scrollY > 0) {
        hideNavigation();
    }
});

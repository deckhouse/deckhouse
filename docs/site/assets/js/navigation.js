$(window).on('load', function() {
    const navigationContainer = document.querySelector('.navigation__container');

    if (navigationContainer) {
        if (window.innerWidth > 1024) {
            navigationContainer.classList.add('fixed-navigation');
            const sidebarAndToc = document.querySelectorAll('.sidebar__wrapper-inner');
            let lastScroll = window.scrollY;
            const headerHeight = document.querySelector('.header-container').offsetHeight;
            const navigationHeight = navigationContainer.offsetHeight;
            const newTop = headerHeight + navigationHeight;
            const tocSidebarLinks = document.querySelectorAll('.toc-sidebar__item-link');

            let isScroll = true;

            function hideNavigation() {
                navigationContainer.classList.add('hidden');
                lastScroll = window.scrollY;
                sidebarAndToc.forEach(e => {
                    e.classList.remove('top');
                    e.style.removeProperty('--scroll-top');
                });
            }

            function scrollHandler() {
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
                        e.classList.add('top');
                        e.style.setProperty('--scroll-top', `${newTop}px`);
                    });
                }
            }

            tocSidebarLinks.forEach(link => {
                link.addEventListener('click', () => {
                    isScroll = false;
                    hideNavigation();
                    setTimeout(() => {
                        isScroll = true;
                    }, 500);
                });
            });

            window.addEventListener('scroll', scrollHandler);

            if (window.scrollY > 0) {
                hideNavigation();
            }
        } else {
            navigationContainer.classList.remove('fixed-navigation');
        }
    }
});

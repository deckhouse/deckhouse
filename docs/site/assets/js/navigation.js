document.addEventListener('DOMContentLoaded', () => {
    const navigationContainer = document.querySelector('.navigation__container');

    if (navigationContainer) {
        const sidebarAndToc = document.querySelectorAll('.sidebar__wrapper-inner');
        const tocSidebarLinks = document.querySelectorAll('.toc-sidebar__item-link');

        let lastScroll = window.scrollY;
        let isScroll = true;
        let isDesktop = false;

        function hideNavigation() {
            navigationContainer.classList.add('hidden');
            lastScroll = window.scrollY;
            sidebarAndToc.forEach(e => {
                e.classList.remove('top');
                e.style.removeProperty('--scroll-top');
            });
        }

        function showNavigation() {
            navigationContainer.classList.remove('hidden');
            lastScroll = window.scrollY;

            const headerHeight = document.querySelector('.header-container')?.offsetHeight || 0;
            const navigationHeight = navigationContainer.offsetHeight;
            const newTop = headerHeight + navigationHeight;

            sidebarAndToc.forEach(e => {
                e.classList.add('top');
                e.style.setProperty('--scroll-top', `${newTop}px`);
            });
        }

        function scrollHandler() {
            if (!isScroll || !isDesktop) return;

            const scrollWindowTop = window.scrollY;
            if (scrollWindowTop > lastScroll) {
                hideNavigation();
            } else {
                showNavigation();
            }
        }

        function applyLayout() {
            if (window.innerWidth > 1024) {
                if (!isDesktop) {
                    isDesktop = true;
                    window.addEventListener('scroll', scrollHandler);
                }

                navigationContainer.classList.add('fixed-navigation');
                if (window.scrollY > 0) {
                    hideNavigation();
                } else {
                    showNavigation();
                }
            } else {
                if (isDesktop) {
                    window.removeEventListener('scroll', scrollHandler);
                }

                isDesktop = false;
                navigationContainer.classList.remove('fixed-navigation');
                navigationContainer.classList.remove('hidden');
                sidebarAndToc.forEach(e => {
                    e.classList.remove('top');
                    e.style.removeProperty('--scroll-top');
                });
            }
        }

        tocSidebarLinks.forEach(link => {
            link.addEventListener('click', () => {
                if (!isDesktop) return;

                isScroll = false;
                hideNavigation();
                setTimeout(() => {
                    isScroll = true;
                }, 500);
            });
        });

        applyLayout();
        window.addEventListener('resize', applyLayout);
    }
});

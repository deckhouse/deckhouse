document.addEventListener('DOMContentLoaded', function () {
    const body = document.querySelector('body');
    const content = document.querySelector('.content');
    const hamburgerCollapse = document.querySelector('.hamburger--collapse');
    const headerSidebar = document.querySelector('.header__sidebar');
    const navTrigger = document.querySelector('div .nav__trigger');
    const headerNavList = document.querySelector('ul#bottom-header-nav.nav.header__nav');
    const moduleSidebarNavList = document.querySelector('.header__sidebar .header__sidebar--nav');
    const isModuleHeader = !!document.querySelector('.header-container--module');
    const burgerOverlay = document.createElement('div');
    const modulesName = document.createElement('p');
    modulesName.className = 'module-name';

    let burgerInited = false;
    let mobileSidebarInited = false;
    let cloneSidebar = null;

    const menuItemChildren = document.querySelectorAll('.menu-item-has-children');
    menuItemChildren.forEach(item => {
        item.addEventListener('click', () => {
            const subMenu = item.querySelector('.sub-menu-container');
            subMenu.classList.toggle('sub-menu_open');
        })
    })

    function getNavItemTitle(item) {
        if (!item) return '';
        const link = item.querySelector('a');
        if (link) return link.firstChild.textContent.trim();
        return item.firstChild.textContent.trim();
    }

    function getMobileNavList() {
        if (isModuleHeader && moduleSidebarNavList) return moduleSidebarNavList;
        return headerNavList;
    }

    function activeMobileItem() {
        if (window.innerWidth >= 1024) return null;
        const mobileNavList = getMobileNavList();
        if (!mobileNavList) return null;

        const activeNavMobile = mobileNavList.querySelector('li.active-mobile');
        if (activeNavMobile) {
            if(navTrigger) {
                navTrigger.textContent = getNavItemTitle(activeNavMobile);
            }
            return activeNavMobile;
        }

        const activeNav = isModuleHeader
            ? (
                mobileNavList.querySelector('.header__navigation-item.header__navigation-item_active') ||
                mobileNavList.querySelector('.header__navigation-item.active') ||
                mobileNavList.querySelector('.header__navigation-item')
            )
            : (
                mobileNavList.querySelector('.header__navigation-item.active') ||
                mobileNavList.querySelector('.header__navigation-item')
            );
        if (!activeNav) return null;

        if (isModuleHeader) {
            let originActiveClass = 'active';
            if (activeNav.classList.contains('header__navigation-item_active')) {
                originActiveClass = 'header__navigation-item_active';
            } else if (activeNav.classList.contains('active')) {
                originActiveClass = 'active';
            }
            activeNav.dataset.mobileActiveOrigin = originActiveClass;
        }

        activeNav.classList.remove('active', 'header__navigation-item_active');
        activeNav.classList.add('active-mobile');
        if(navTrigger) {
            navTrigger.textContent = getNavItemTitle(activeNav);
        }
        return activeNav;
    }

    function desktopActiveItem() {
        const mobileActiveItems = document.querySelectorAll('.header__navigation-item.active-mobile');
        mobileActiveItems.forEach(function (item) {
            item.classList.remove('active-mobile');

            if (isModuleHeader) {
                item.classList.remove('active', 'header__navigation-item_active');
                const originActiveClass = item.dataset.mobileActiveOrigin;
                if (originActiveClass === 'header__navigation-item_active') {
                    item.classList.add('header__navigation-item_active');
                } else {
                    item.classList.add('active');
                }
                delete item.dataset.mobileActiveOrigin;
            } else {
                item.classList.add('active');
            }
        });
    }

    function updateMobileNavScrollState() {
        if (!headerNavList) return;

        if (window.innerWidth >= 1024 || !headerNavList.classList.contains('active')) {
            headerNavList.style.maxHeight = '';
            headerNavList.style.overflowY = '';
            headerNavList.style.overflowX = '';
            headerNavList.style.webkitOverflowScrolling = '';
            return;
        }

        const navRect = headerNavList.getBoundingClientRect();
        const viewportPadding = 12;
        const availableHeight = Math.floor(window.innerHeight - navRect.top - viewportPadding);
        const maxHeight = Math.max(140, availableHeight);

        headerNavList.style.maxHeight = `${maxHeight}px`;
        headerNavList.style.overflowY = 'auto';
        headerNavList.style.overflowX = 'hidden';
        headerNavList.style.webkitOverflowScrolling = 'touch';
    }

    function getLastPathname() {
        const pathname = window.location.pathname;
        const lastSlash = pathname.lastIndexOf('/');

        if (lastSlash === pathname.length - 1) {
            const penultimateSlash = pathname.substring(0, lastSlash).lastIndexOf('/');
            return pathname.substring(penultimateSlash + 1, lastSlash) + '/';
        }

        const partsPathname = pathname.split('/');
        return partsPathname.slice(-3).join('/');
    }

    function getActiveSidebarLink() {
        if (!cloneSidebar) return null;

        let lastPathname = getLastPathname();
        if (lastPathname.lastIndexOf('v1') !== 1) {
            lastPathname = lastPathname.replace(/v1/g, '.');
        }

        const activeLinkByPath = cloneSidebar.querySelector(`li.sidebar__submenu-item.active a[href$="${lastPathname}"]`);
        if (activeLinkByPath) return activeLinkByPath;

        const activeLink = Array.from(
            cloneSidebar.querySelectorAll('.sidebar__submenu-item.active a')
        ).pop();
        if (activeLink) return activeLink;

        return cloneSidebar.querySelector(`a[href$="${lastPathname}"]`);
    }

    function getTopActiveLink(element, sidebar) {
        let top = 0;

        while (element && element !== sidebar) {
            top += element.offsetTop;
            element = element.offsetParent;
        }

        return top;
    }

    function getModuleNameFromHref() {
        const href = window.location.href || '';
        const match = href.match(/\/modules\/([^/]+)/);
        return match ? decodeURIComponent(match[1]) : '';
    }

    function updateModulesLinkLabel(activeNavItem) {
        if (!activeNavItem) return;
        const modulesLink = activeNavItem.querySelector('a[href="/modules/"], a[href="/modules"]');
        if (!modulesLink) return;

        const moduleName = getModuleNameFromHref();
        if (!moduleName) return;

        modulesName.textContent = `(${moduleName})`;
        modulesLink.appendChild(modulesName);
    }

    function scrollContainerToActiveLink(container, activeLink) {
        if (!container || !activeLink) return;

        const activeLinkTop = getTopActiveLink(activeLink, container);
        const activeLinkTopForScroll = activeLinkTop;

        if (
            activeLinkTopForScroll < container.scrollTop ||
            (activeLinkTopForScroll + activeLink.offsetHeight) > (container.scrollTop + container.clientHeight)
        ) {
            container.scrollTo({
                top: activeLinkTop,
                behavior: 'smooth'
            });
        }
    }

    function scrollMobileSidebarToActive() {
        const activeLink = getActiveSidebarLink();
        if (!activeLink) return;

        if (headerNavList && headerNavList.classList.contains('header__nav--doc-modal')) {
            scrollContainerToActiveLink(headerNavList, activeLink);
        }
    }

    function openBurgerSidebar() {
        if (!headerSidebar) return;
        closeNavModal();
        hamburgerCollapse.classList.add('show');
        headerSidebar.classList.add('show');
        burgerOverlay.classList.add('sidebar-overlay');
        content.appendChild(burgerOverlay);
        body.classList.add('sidebar-opened');
    }

    function closeBurgerSidebar() {
        closeNavModal();
        if (headerSidebar) headerSidebar.classList.remove('show');
        if (burgerOverlay.parentNode) burgerOverlay.parentNode.removeChild(burgerOverlay);
        if (body) body.classList.remove('sidebar-opened');
        if (hamburgerCollapse) hamburgerCollapse.classList.remove('show');
    }

    function initBurger() {
        if (window.innerWidth >= 1024 || burgerInited) return;
        burgerInited = true;

        if (hamburgerCollapse) {
            hamburgerCollapse.addEventListener('click', function () {
                if (window.innerWidth < 1024) {
                    const filterBlock = document.querySelector('.filter__block');
                    if (filterBlock && filterBlock.classList.contains('show')) {
                        filterBlock.classList.remove('show');
                        hamburgerCollapse.classList.remove('show');
                        if (body) body.classList.remove('sidebar-opened');
                        const filterOverlay = document.querySelector('.filter__sidebar-overlay');
                        if (filterOverlay && filterOverlay.parentNode) {
                            filterOverlay.parentNode.removeChild(filterOverlay);
                        }
                        return;
                    }
                }
                if (!headerSidebar) return;
                if (headerSidebar.classList.contains('show')) {
                    closeBurgerSidebar();
                } else {
                    openBurgerSidebar();
                }
            });
        }

        window.addEventListener('click', function (e) {
            if (!headerSidebar || !headerSidebar.classList.contains('show')) return;
            const clickedInsideSidebar = headerSidebar.contains(e.target);
            const clickedHamburger = hamburgerCollapse && hamburgerCollapse.contains(e.target);
            if (!clickedInsideSidebar && !clickedHamburger) {
                closeBurgerSidebar();
            }
        });
    }

    function closeNavModal() {
        if (cloneSidebar) {
            cloneSidebar.classList.remove('header__sidebar-nav--show');
            cloneSidebar.setAttribute('aria-hidden', 'true');
        }

        const activeNavMobile = document.querySelector('li.active-mobile');
        if (activeNavMobile) activeNavMobile.classList.remove('header__navigation-item--open');

        if (headerNavList) headerNavList.classList.remove('active', 'header__nav--doc-modal');
        updateMobileNavScrollState();
        if (body) body.classList.remove('sidebar-opened');
        if (navTrigger) navTrigger.classList.remove('rotated');

        const navOverlay = document.querySelector('.header__nav-overlay');
        if (navOverlay && navOverlay.parentNode) navOverlay.parentNode.removeChild(navOverlay);
    }

    function ensureOverlay() {
        if (document.querySelector('.header__nav-overlay')) return;
        const overlay = document.createElement('div');
        overlay.className = 'sidebar-overlay header__nav-overlay';
        body.appendChild(overlay);
        overlay.addEventListener('click', function (e) {
            if (e.target !== overlay) return;
            closeNavModal();
        });
    }

    function initMobileSidebar() {
        if (window.innerWidth >= 1024) return;

        const activeNavMobile = activeMobileItem();
        if (!activeNavMobile || mobileSidebarInited) return;
        mobileSidebarInited = true;

        const contentSidebar = document.querySelector('.layout-sidebar__sidebar .sidebar');

        if (contentSidebar) {
            cloneSidebar = contentSidebar.cloneNode(true);
            cloneSidebar.classList.add('header__sidebar-nav');
            cloneSidebar.setAttribute('id', 'header-doc-mysidebar');
            cloneSidebar.setAttribute('aria-hidden', 'true');
            cloneSidebar.addEventListener('click', function (e) {
                e.stopPropagation();
            });

            activeNavMobile.appendChild(cloneSidebar);
            if (typeof window.$ !== 'undefined' && window.$.fn.navgoco) {
                window.$(cloneSidebar).navgoco({
                    caretHtml: '',
                    accordion: false,
                    openClass: 'active',
                    save: false,
                    cookie: { name: 'navgoco-header-doc', expires: false, path: '/' },
                    slide: { duration: 400, easing: 'swing' }
                });
            }
        }

        const mobileNavList = getMobileNavList();
        if (mobileNavList) {
            mobileNavList.addEventListener('click', function(e) {
                e.stopPropagation();
            });
        }

        if (navTrigger && headerNavList) {
            navTrigger.addEventListener('click', function(e) {
                e.preventDefault();
                e.stopPropagation();
                const isNavOpen = headerNavList.classList.contains('active') ||
                    (cloneSidebar && cloneSidebar.classList.contains('header__sidebar-nav--show'));

                if (isNavOpen) {
                    closeNavModal();
                    return;
                }

                closeBurgerSidebar();
                headerNavList.classList.add('active');
                navTrigger.classList.add('rotated');
                body.classList.add('sidebar-opened');
                ensureOverlay();
                window.requestAnimationFrame(updateMobileNavScrollState);

                if (activeNavMobile && cloneSidebar) {
                    cloneSidebar.classList.add('header__sidebar-nav--show');
                    cloneSidebar.setAttribute('aria-hidden', 'false');
                    activeNavMobile.classList.add('header__navigation-item--open');
                    updateModulesLinkLabel(activeNavMobile);
                    headerNavList.classList.add('header__nav--doc-modal');
                    window.requestAnimationFrame(scrollMobileSidebarToActive);
                }
            });
        }
    }

    function syncHeaderDisplay() {
        if (window.innerWidth < 1024) {
            activeMobileItem();
            initBurger();
            initMobileSidebar();
            updateMobileNavScrollState();
            return;
        }

        modulesName.remove();
        desktopActiveItem();
        closeBurgerSidebar();
        closeNavModal();
    }

    syncHeaderDisplay();
    window.addEventListener('resize', syncHeaderDisplay);
});
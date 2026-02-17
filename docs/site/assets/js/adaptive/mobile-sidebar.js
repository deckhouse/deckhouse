document.addEventListener('DOMContentLoaded', function () {
    if (window.innerWidth >= 1024) return;
    const list = document.querySelector('ul#bottom-header-nav.nav.header__nav');
    const contentSidebar = document.querySelector('.layout-sidebar__sidebar .sidebar');
    const activeList = document.querySelector('li.active-mobile');
    const body = document.querySelector('body');
    const navList = document.querySelector('div .nav__trigger');
    let cloneSidebar = null;

    if (contentSidebar && activeList) {
        cloneSidebar = contentSidebar.cloneNode(true);
        cloneSidebar.classList.add('header__sidebar-nav');
        cloneSidebar.setAttribute('id', 'header-doc-mysidebar');
        cloneSidebar.setAttribute('aria-hidden', 'true');
        cloneSidebar.addEventListener('click', function(e) {
            e.stopPropagation();
        });

        activeList.appendChild(cloneSidebar);
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

    function closeNavModal() {
        if (cloneSidebar) {
            cloneSidebar.classList.remove('header__sidebar-nav--show');
            cloneSidebar.setAttribute('aria-hidden', 'true');
        }
        if (activeList) activeList.classList.remove('header__navigation-item--open');
        if (list) list.classList.remove('active', 'header__nav--doc-modal');
        body.classList.remove('sidebar-opened');
        navList.classList.remove('rotated');
        const overlay = document.querySelector('.header__nav-overlay');
        if (overlay && overlay.parentNode) overlay.parentNode.removeChild(overlay);
    }

    function ensureOverlay() {
        if (document.querySelector('.header__nav-overlay')) return;
        const overlay = document.createElement('div');
        overlay.className = 'sidebar-overlay header__nav-overlay';
        body.appendChild(overlay);
        overlay.addEventListener('click', function(e) {
            if (e.target !== overlay) return;
            closeNavModal();
        });
    }

    if (list) {
        list.addEventListener('click', function(e) {
            e.stopPropagation();
        });
    }

    navList.addEventListener('click', function(e) {
        e.stopPropagation();
        const willOpen = !list.classList.contains('active');
        list.classList.toggle('active');
        navList.classList.toggle('rotated');
        if (willOpen) {
            body.classList.add('sidebar-opened');
            ensureOverlay();
        } else {
            closeNavModal();
        }
    });

    if (activeList && cloneSidebar) {
        activeList.addEventListener('click', function(e) {
            e.preventDefault();
            const isOpening = !cloneSidebar.classList.contains('header__sidebar-nav--show');
            cloneSidebar.classList.toggle('header__sidebar-nav--show');
            cloneSidebar.setAttribute('aria-hidden', !isOpening);
            if (isOpening) {
                activeList.classList.add('header__navigation-item--open');
                list.classList.add('header__nav--doc-modal');
                body.classList.add('sidebar-opened');
                ensureOverlay();
            } else {
                activeList.classList.remove('header__navigation-item--open');
                list.classList.remove('header__nav--doc-modal');
                body.classList.remove('sidebar-opened');
            }
        });
    }
});

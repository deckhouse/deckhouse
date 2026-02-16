document.addEventListener('DOMContentLoaded', function () {
    if (window.innerWidth < 1024) {
        const headerNav = document.querySelector('.header__navigation-item.active');
        headerNav.classList.remove('active');
        headerNav.classList.add('active-mobile');
        const hamburgerCollapse = document.querySelector('.hamburger--collapse');
        const headerSidebar = document.querySelector('.header__sidebar');
        const content = document.querySelector('.content');
        const overlay = document.createElement('div');
        const body = document.querySelector('body');
        const navList = document.querySelector('div .nav__trigger');
        const activeList = document.querySelector('li.active-mobile');
        
        if(activeList) {
            navList.textContent = activeList.textContent;
        }

        hamburgerCollapse.addEventListener('click', function() {
            headerSidebar.classList.toggle('show');

            if(headerSidebar.classList.contains('show')) {
                overlay.classList.add('sidebar-overlay');
                content.appendChild(overlay);
                body.classList.add('sidebar-opened');
            } else {
                if(overlay) {
                    content.removeChild(overlay);
                    body.classList.remove('sidebar-opened');
                }
            }
        })

        window.addEventListener('click', function(e) {
            if(!e.target.matches('.hamburger--collapse')) {
                if(headerSidebar.classList.contains('show')) {
                    headerSidebar.classList.remove('show');
                    if(overlay) {
                        content.removeChild(overlay);
                        body.classList.remove('sidebar-opened');
                    }
                }
            }
        })

        const list = document.querySelector('ul#bottom-header-nav.nav.header__nav');
        const contentSidebar = document.querySelector('.layout-sidebar__sidebar .sidebar');
        let cloneSidebar = null;

        if (contentSidebar && activeList) {
            cloneSidebar = contentSidebar.cloneNode(true);
            cloneSidebar.classList.add('header__sidebar');
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
                cloneSidebar.classList.remove('header__sidebar--show');
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
                const isOpening = !cloneSidebar.classList.contains('header__sidebar--show');
                cloneSidebar.classList.toggle('header__sidebar--show');
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

        $('#language-switch').each(function() {
            let pageDomain = window.location.hostname;
            if (window.location.pathname.startsWith('/ru/')) {
            $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('/ru/', '/en/')}'`)
            $(this).attr('checked', 'checked');
            } else if (window.location.pathname.startsWith('/en/')) {
            $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('/en/', '/ru/')}'`)
            $(this).removeAttr('checked', 'checked');
            } else {
            switch (pageDomain) {
                case 'deckhouse.io':
                $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('deckhouse.io', 'deckhouse.ru')}'`)
                $(this).removeAttr('checked', 'checked');
                break;
                case 'deckhouse.ru':
                $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('deckhouse.ru', 'deckhouse.io')}'`)
                $(this).attr('checked', 'checked');
                break;
                case 'ru.localhost':
                $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('ru.localhost', 'localhost')}'`)
                $(this).attr('checked', 'checked');
                break;
                case 'localhost':
                $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('localhost', 'ru.localhost')}'`)
                $(this).removeAttr('checked', 'checked');
                break;
                default:
                if (pageDomain.includes('deckhouse.ru.')) {
                    $(this).attr('onclick', `javascript:location.href='${ window.location.href.replace('deckhouse.ru.', 'deckhouse.')}'`)
                    $(this).attr('checked', 'checked');
                } else if (pageDomain.includes('deckhouse.')) {
                    $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('deckhouse.', 'deckhouse.ru.')}'`)
                    $(this).removeAttr('checked', 'checked');
                }
            }
            }
        });

        const titleHidden = document.querySelectorAll('h2');
        titleHidden.forEach(header => {
            let nextElements = header.nextElementSibling;
            const elementsToggle = [];

            while (nextElements && nextElements.tagName !== 'H2') {
                elementsToggle.push(nextElements)
                nextElements = nextElements.nextElementSibling;
            };
            
            header.addEventListener('click', () => {
                header.classList.toggle('closed-header');
                elementsToggle.forEach(element => {
                    element.classList.toggle('hidden');
                })
            })
        })
    }
})
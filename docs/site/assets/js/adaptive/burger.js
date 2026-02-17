document.addEventListener('DOMContentLoaded', function () {
    if (window.innerWidth >= 1024) return;
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
});

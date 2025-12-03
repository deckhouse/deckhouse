document.addEventListener('DOMContentLoaded', function () {
    const navigationContainer = document.querySelector('.navigation__container');
    const sidebarAndToc = document.querySelectorAll('.sidebar__wrapper-inner');
    let lastScroll = window.scrollY;
    const headerHeight = document.querySelector('.header-container').offsetHeight;
    const navigationHeight = navigationContainer.offsetHeight;
    const newTop = headerHeight + navigationHeight;
    const range = 10;

    if(navigationContainer) {
        window.addEventListener('scroll', function() {
            const scrollWindowTop = window.scrollY;
            const scrollDelta = scrollWindowTop - lastScroll;

                if(scrollWindowTop > lastScroll) {
                    navigationContainer.classList.add('hidden');
                    lastScroll = scrollWindowTop;
                    sidebarAndToc.forEach(e => {
                        e.classList.remove('top');
                        e.style.removeProperty('--scroll-top');
                    })
                } else {
                    if(Math.abs(scrollDelta) > range) {
                        navigationContainer.classList.remove('hidden'); 
                        lastScroll = scrollWindowTop;
                        sidebarAndToc.forEach(e => {
                            e.classList.add('top');
                            e.style.setProperty('--scroll-top', `${newTop}px`);
                        })
                    }

                }
        })
    }
})
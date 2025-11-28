document.addEventListener('DOMContentLoaded', function () {
    const navigationContainer = document.querySelector('.navigation__container');
    let lastScroll = window.scrollY;

    if(navigationContainer) {
        window.addEventListener('scroll', function() {
            const scrollWindowTop = window.scrollY;

                if(scrollWindowTop > lastScroll) {
                    navigationContainer.classList.add('hidden');
                    lastScroll = scrollWindowTop;
                } else {
                      navigationContainer.classList.remove('hidden'); 
                      lastScroll = scrollWindowTop;
                }
        })
    }
})
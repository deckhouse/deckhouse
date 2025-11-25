document.addEventListener('DOMContentLoaded', function () {
    const navigationContainer = document.querySelector('.navigation__container');
    let lastScroll = window.scrollY;
    const range = 10;

    if(navigationContainer) {
        window.addEventListener('scroll', function() {
            const scrollWindowTop = window.scrollY;
            const scrollDelta = scrollWindowTop - lastScroll;

                if(scrollWindowTop > lastScroll) {
                    navigationContainer.classList.add('hidden');
                    lastScroll = scrollWindowTop;
                } else {
                    if(Math.abs(scrollDelta) > range) {
                      navigationContainer.classList.remove('hidden'); 
                      lastScroll = scrollWindowTop; 
                    }
                }
        })
    }
})
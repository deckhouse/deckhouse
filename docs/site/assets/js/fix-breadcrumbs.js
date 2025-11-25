document.addEventListener('DOMContentLoaded', function () {
    const breadcrumbsContainer = document.querySelector('.navigation__container');
    let lastScrollWindowTop = 0;

    if(breadcrumbsContainer) {
        window.addEventListener('scroll', function() {
            let scrollWindowTop = window.pageYOffset || document.documentElement.scrollTop;

            if(scrollWindowTop > lastScrollWindowTop) {
                breadcrumbsContainer.classList.add('hidden');
            } else {
                breadcrumbsContainer.classList.remove('hidden');
            }

            lastScrollWindowTop = scrollWindowTop <= 0 ? 0 : scrollWindowTop;
        })
    }
})
document.addEventListener('DOMContentLoaded', function () {
    if(window.innerWidth >= 1024) {
        return;
    }

    const images = document.querySelectorAll('.docs img');
    const content = document.querySelector('.content');
    const body = document.querySelector('body');
    const  overlay = document.createElement('div');

    function createCarousel(images) {
        const parentFirstImages = images[0].parentNode;

        const carousel = document.createElement('div');
        carousel.classList.add('carousel');

        const carouselImages = document.createElement('div');
        carouselImages.classList.add('carousel__images');

        const carouselControl = document.createElement('div');
        carouselControl.classList.add('carousel__control');

        const previousBtn = document.createElement('div');
        previousBtn.classList.add('carousel__control--previous');
        const nextBtn = document.createElement('div');
        nextBtn.classList.add('carousel__control--next');

        const imgCount = document.createElement('span');
        imgCount.textContent = images.length;

        carouselControl.appendChild(previousBtn);
        carouselControl.appendChild(imgCount);
        carouselControl.appendChild(nextBtn);

        carousel.appendChild(carouselImages);
        carousel.appendChild(carouselControl);

        images.forEach(img => {
            carouselImages.appendChild(img);
        })

        parentFirstImages.appendChild(carousel);

        let counter = 0;
        const imageWidth = carousel.offsetWidth;

        images.forEach(img => {
            img.style.minWidth = `${imageWidth}px`;
        })

        nextBtn.addEventListener('click', () => {
            counter++;
            if(counter >= images.length) {
                counter = 0;
            }
            carouselImages.style.transform = `translateX(${-imageWidth * counter}px)`;
        })

        previousBtn.addEventListener('click', () => {
            counter--;
            if(counter < 0) {
                counter = images.length - 1;
            }
            carouselImages.style.transform = `translateX(${-imageWidth * counter}px)`;
        })
    }

    let imagesGroup = [];
    let carrentGroup = [];

    for(let i = 0; i < images.length; i++) {
        carrentGroup.push(images[i]);
        if(i === images.length - 1) {
            if(carrentGroup.length > 1) {
                imagesGroup.push(carrentGroup);
            }
            carrentGroup = [];
        } else if(images[i + 1].tagName !== 'IMG') {
            if(carrentGroup.length > 1) {
                imagesGroup.push(carrentGroup);
            }
            carrentGroup = [];
        }
    }

    imagesGroup.forEach(group => {
        createCarousel(group);
    })

    const imagesAll = document.querySelectorAll('.docs img');

    imagesAll.forEach(img => {
        const container = document.createElement('div');
        container.classList.add('img__container');
        img.parentNode.insertBefore(container, img);
        container.appendChild(img);

        const zoomIcon = document.createElement('div');
        zoomIcon.classList.add('img__container--zoom');
        container.appendChild(zoomIcon);

        let isZoom = false;

        img.onload = function() {
            zoomIcon.style.setProperty('--img-height', img.offsetHeight + 'px');
            zoomIcon.style.setProperty('--img-width', img.offsetWidth + 'px');
        }

        function toggleZoom() {
            if(!isZoom) {
                img.classList.add('zoom');
                overlay.classList.add('sidebar-overlay');
                content.appendChild(overlay);
                body.classList.add('sidebar-opened');
                zoomIcon.style.setProperty('--img-height', img.offsetHeight + 'px');
                zoomIcon.style.setProperty('--img-width', img.offsetWidth + 'px');
            } else {
                img.classList.remove('zoom');
                content.removeChild(overlay);
                body.classList.remove('sidebar-opened');
            }

            isZoom = !isZoom;
        }

        zoomIcon.addEventListener('click', (e) => {
            e.stopPropagation();
            toggleZoom();
        })

        img.addEventListener('click', () => {
            if(isZoom) {
                toggleZoom();
            }
        })

        overlay.addEventListener('click', () => {
            if(isZoom) {
                toggleZoom();
            }
        })
    })
})
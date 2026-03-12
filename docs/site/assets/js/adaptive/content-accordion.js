document.addEventListener('DOMContentLoaded', function () {
    const titleHidden = document.querySelectorAll('.docs h2');

    titleHidden.forEach(header => {
        let nextElements = header.nextElementSibling;
        const elementsToggle = [];

        while (nextElements && nextElements.tagName !== 'H2') {
            elementsToggle.push(nextElements);
            nextElements = nextElements.nextElementSibling;
        };

        header.addEventListener('click', () => {
            if (window.innerWidth < 1024) {
                header.classList.toggle('closed-header');
                elementsToggle.forEach(element => {
                    element.classList.toggle('hidden');
                })
            }

        })
    });

    window.addEventListener('resize', () => {
        if (window.innerWidth >= 1024) {
            titleHidden.forEach(header => {
                header.classList.remove('closed-header');
            });
            document.querySelectorAll('.docs .hidden').forEach(el => {
                el.classList.remove('hidden');
            });
        }
    });
});

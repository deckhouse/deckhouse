document.addEventListener('DOMContentLoaded', function () {
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
});

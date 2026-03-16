document.addEventListener('DOMContentLoaded', function () {
    const faqContainer = document.querySelector('.docs.faq__container');
    const expandButton = document.querySelector('.show__containers--expand');
    const collapseButton = document.querySelector('.show__containers--collapse');

    if (!faqContainer || !expandButton || !collapseButton) {
        return;
    }

    const faqTitle = faqContainer.querySelectorAll('h3');
    collapseButton.classList.add('active');
    const faqContent = faqContainer.querySelectorAll('h3 + div');

    function findContent(element) {
        let content = element.nextElementSibling;

        while(content) {
            if(content.tagName === 'DIV') {
                return content;
            }
            content = content.nextElementSibling;
        }
        return null;
    };

    expandButton.addEventListener('click', () => {
        expandButton.classList.remove('active');
        collapseButton.classList.add('active');
        faqTitle.forEach(title => {
            title.classList.remove('hide');
        });
        faqContent.forEach(content => {
            content.classList.remove('hidden');
        });
    });

    collapseButton.addEventListener('click', () => {
        expandButton.classList.add('active');
        collapseButton.classList.remove('active');
        faqTitle.forEach(title => {
            title.classList.add('hide');
        });
        faqContent.forEach(content => {
            content.classList.add('hidden');
        });
    });

    faqTitle.forEach(title => {
        const content = findContent(title);
        
        title.addEventListener('click', () => {
            if (!content) {
                return;
            };
            title.classList.toggle('hide');
            content.classList.toggle('hidden');
        });
    });
})
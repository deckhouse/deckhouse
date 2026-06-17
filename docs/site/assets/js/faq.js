document.addEventListener('DOMContentLoaded', function () {
    const faqContainer = document.querySelector('.docs.faq__container');
    const expandButton = document.querySelector('.show__containers--expand');
    const collapseButton = document.querySelector('.show__containers--collapse');

    if (!faqContainer || !expandButton || !collapseButton) {
        return;
    }

    const faqTitle = faqContainer.querySelectorAll('h3');
    expandButton.classList.add('active');
    const faqContent = faqContainer.querySelectorAll('h3 + div');
    const sectionMap = new Map();

    function findContent(element) {
        let content = element.nextElementSibling;

        while(content) {
            if(content.tagName === 'DIV') {
                return content;
            }
            content = content.nextElementSibling;
        }
        return null;
    }

    function getScrollOffset() {
        const header = document.querySelector('header');
        const navigation = document.querySelector('.navigation__container');
        const headerHeight = header ? header.offsetHeight : 0;
        const navigationHeight = navigation && !navigation.classList.contains('hidden') ? navigation.offsetHeight : 0;

        return headerHeight + navigationHeight + 10;
    }

    function alignTitleWithViewport(title) {
        const scrollTop = title.getBoundingClientRect().top + window.scrollY - getScrollOffset();
        window.scrollTo(0, Math.max(0, scrollTop));
    }

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

    function showSectionByHash(onlyTarget) {
        const hash = decodeURIComponent(window.location.hash.replace('#', ''));
        const title = hash ? document.getElementById(hash) : null;

        if (!title || !sectionMap.has(title)) {
            return;
        }

        if (onlyTarget) {
            sectionMap.forEach((contentItem, titleItem) => {
                const isTarget = titleItem === title;

                titleItem.classList.toggle('hide', !isTarget);
                if (contentItem) {
                    contentItem.classList.toggle('hidden', !isTarget);
                }
            });
            requestAnimationFrame(() => alignTitleWithViewport(title));
            return;
        }

        const content = sectionMap.get(title);
        title.classList.remove('hide');
        if (content) {
            content.classList.remove('hidden');
        }

        requestAnimationFrame(() => alignTitleWithViewport(title));
    }

    faqTitle.forEach(title => {
        const content = findContent(title);

        if (!content) {
            return;
        }

        sectionMap.set(title, content);
        title.classList.add('hide');
        content.classList.add('hidden');

        title.addEventListener('click', () => {
            title.classList.toggle('hide');
            content.classList.toggle('hidden');

            requestAnimationFrame(() => alignTitleWithViewport(title));
        });
    });

    showSectionByHash(true);

    window.addEventListener('hashchange', () => {
        showSectionByHash(false);
    });
});

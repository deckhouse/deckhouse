// Collapsible h3 sections: FAQ pages and blocks marked with the collapsible-sections class
// (for example, patch releases on the Release notes page).
document.addEventListener('DOMContentLoaded', function () {
    const containers = document.querySelectorAll('.docs.faq__container, .collapsible-sections');

    if (!containers.length) {
        return;
    }

    const expandButton = document.querySelector('.show__containers--expand');
    const collapseButton = document.querySelector('.show__containers--collapse');
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

    function setAllCollapsed(collapsed) {
        sectionMap.forEach((content, title) => {
            title.classList.toggle('hide', collapsed);
            content.classList.toggle('hidden', collapsed);
        });
    }

    if (expandButton && collapseButton) {
        expandButton.classList.add('active');

        expandButton.addEventListener('click', () => {
            expandButton.classList.remove('active');
            collapseButton.classList.add('active');
            setAllCollapsed(false);
        });

        collapseButton.addEventListener('click', () => {
            expandButton.classList.add('active');
            collapseButton.classList.remove('active');
            setAllCollapsed(true);
        });
    }

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

    containers.forEach(container => {
        container.querySelectorAll('h3').forEach(title => {
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
    });

    showSectionByHash(true);

    window.addEventListener('hashchange', () => {
        showSectionByHash(false);
    });
});

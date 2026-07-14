document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('.tabs-block').forEach(function (tabsBlock) {
        const tabsTitles = tabsBlock.querySelectorAll('.tabs__item--title');
        const tabsDescr = tabsBlock.querySelectorAll('.tabs__item--descr');

        if (!tabsTitles.length || !tabsDescr.length) return;

        function activeTab(index) {
            tabsTitles.forEach(title => title.classList.remove('active'));
            tabsDescr.forEach(descr => descr.classList.remove('active'));

            tabsTitles[index].classList.add('active');
            tabsDescr[index].classList.add('active');
        }

        tabsTitles.forEach(title => {
            title.addEventListener('click', () => {
                const titleIndex = parseInt(title.dataset.index, 10) - 1;

                if (!title.classList.contains('active')) {
                    activeTab(titleIndex);
                }
            });
        });

        activeTab(0);
    });
});
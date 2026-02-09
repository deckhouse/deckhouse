document.addEventListener('DOMContentLoaded', function () {
    const channelListTitle = document.querySelectorAll('.channel__item--title');
    const channelListDescr = document.querySelectorAll('.channel__item--descr');

    if (!channelListTitle.length || !channelListDescr.length) return;

    function activeTab(index) {
        channelListTitle.forEach(title => title.classList.remove('active'));
        channelListDescr.forEach(descr => descr.classList.remove('active'));

        channelListTitle[index].classList.add('active');
        channelListDescr[index].classList.add('active');
    }

    channelListTitle.forEach(title => {
        title.addEventListener('click', () => {
            const titleIndex = parseInt(title.dataset.index, 10) - 1;

            if (!title.classList.contains('active')) {
                activeTab(titleIndex);
            }
        });
    });

    activeTab(0);
});
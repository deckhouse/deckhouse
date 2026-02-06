document.addEventListener('DOMContentLoaded', function () {
    const channelListTitle = document.querySelectorAll('.channel__list--title');
    const channelListDescr = document.querySelectorAll('.channel__list--descr');

    function activeTab(index) {
        channelListTitle.forEach(title => title.classList.remove('active'));
        channelListDescr.forEach(descr => descr.classList.remove('active'));

        channelListTitle[index].classList.add('active');
        channelListDescr[index].classList.add('active');
    }

    channelListTitle.forEach(title => {
        title.addEventListener('click', () => {
            const index = parseInt(title.dataset.index);
            console.log(index)

            if(!title.classList.contains('active')) {
                activeTab(index);
            }
        });
    });

    activeTab(0);
});
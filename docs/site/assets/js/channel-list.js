document.addEventListener('DOMContentLoaded', function () {
    const channelListTitle = document.querySelectorAll('.channel__list--title');
    console.log(channelListTitle)

    channelListTitle.forEach(title => {
        title.addEventListener('click', function() {
            if(title.classList.contains('show')) {
                return;
            }
            const description = this.nextElementSibling;

            channelListTitle.forEach(otherTitle => {
                if(otherTitle !== title) {
                    const otherDescription = otherTitle.nextElementSibling;
                    otherDescription.classList.remove('active');
                    otherTitle.classList.remove('show');
                }
            });

            if(description.classList.contains('active')) {
                description.classList.remove('active');
                title.classList.remove('show');
            } else {
                description.classList.add('active');
                title.classList.add('show');
            }
        })
    })
})
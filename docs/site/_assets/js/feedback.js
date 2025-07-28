document.addEventListener('DOMContentLoaded', function () {
    const likeIcon = document.querySelector('.icon__like');
    const dislikeIcon = document.querySelector('.icon__dislike');
    const accessModal = document.querySelector('.window__feedback--access');
    const formModal = document.querySelector('.window__feedback--form');
    const moreDetailed = formModal.querySelector('.button');
    const closeBtn = document.querySelectorAll('.modal-window__close-btn');

    let accessModalTimeout;

    function showAccessModal() {
        accessModal.style.display = 'flex';
        accessModalTimeout = setTimeout(hideAccessModal, 10000);
    }

    function hideAccessModal() {
        accessModal.style.display = 'none';
    }

    likeIcon.addEventListener('click', function() {
        likeIcon.classList.add('active');
        dislikeIcon.classList.remove('active');

        showAccessModal();
    })

    dislikeIcon.addEventListener('click', function() {
        dislikeIcon.classList.add('active');
        likeIcon.classList.remove('active');

        formModal.style.display = 'flex';
    })

    closeBtn.forEach(btn => {
        btn.addEventListener('click', function(e) {
            e.preventDefault();
            accessModal.style.display = 'none';
            formModal.style.display = 'none';
        })
    })

    moreDetailed.addEventListener('click', function(e) {
        e.preventDefault();
        const checkbox = formModal.querySelectorAll('.checkbox:checked');
        if(checkbox) {
            console.log(checkbox.values);
        } else {
            console.log('не выбран');
            return;
        }

        formModal.style.display = 'none';
        showAccessModal();
    })

    window.addEventListener('beforeunload', function() {
        this.clearTimeout(accessModalTimeout);
    })
})
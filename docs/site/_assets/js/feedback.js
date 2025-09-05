document.addEventListener('DOMContentLoaded', function () {
    const likeIcon = document.querySelector('.icon__like');
    const dislikeIcon = document.querySelector('.icon__dislike');
    const accessModal = document.querySelector('.window__feedback--access');
    const formModal = document.querySelector('.window__feedback--form');
    const moreDetailed = formModal.querySelector('.button');
    const closeBtn = document.querySelectorAll('.modal-window__close-btn');
    const detailedInput = document.querySelector('.more-detailed');
    const currentUrl = window.location.href;
    const cookieName = 'userFeedback';

    let accessModalTimeout;

    async function getUserIp() {
        try {
            const res = await fetch('https://api.ipify.org?format=json');
            const data = await res.json();
            return data.ip;
        } catch(error) {
            return null;
        }
    };

    function setCookie(name, value, days) {
        const date = new Date();
        date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
        const expires = 'expires=' + date.toUTCString();
        document.cookie = name + '=' + encodeURIComponent(JSON.stringify(value)) + ';' + expires + ';path=/';
    }

    function getCookie(name) {
        const cookieName = name + '=';
        const decode = decodeURIComponent(document.cookie);
        const cookieArray = decode.split(';');

        for(let i = 0; i < cookieArray.length; i++) {
            let cookie = cookieArray[i];
            while (cookie.charAt(0) === ' ') {
                cookie = cookie.substring(1);
            }
            if(cookie.indexOf(cookieName) === 0) {
                try {
                    return JSON.parse(cookie.substring(cookieName.length, cookie.length));
                } catch(error) {
                    return null;
                }
            }
        }
        return null;
    }

    function generateUUID() {
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
            const r = Math.floor(Math.random() * 16);
            const v = c === 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
    }

    let cookieUserData;

    (async function initUserData() {
        cookieUserData = getCookie(cookieName);

        if(!cookieUserData) {
            cookieUserData = { cookieUserId: generateUUID(), cookieUserIp: null,  pages: {}};
        }

        if(!cookieUserData.cookieUserId) {
            cookieUserData.cookieUserId = generateUUID();
        }

        if(!cookieUserData.cookieUserIp) {
            const ip = await getUserIp();
            cookieUserData.cookieUserIp = ip;
            setCookie(cookieName, cookieUserData, 365);
        }

        buttonState();
    })();

    function showAccessModal() {
        accessModal.style.display = 'flex';
        clearTimeout(accessModalTimeout);
        accessModalTimeout = setTimeout(hideAccessModal, 10000);
    }

    function hideAccessModal() {
        accessModal.style.display = 'none';
    }

    function showFormModal() {
        formModal.style.display = 'flex';
    }

    function hideFormModal() {
        formModal.style.display = 'none';
        formModal.querySelectorAll('.checkbox').forEach(checkbox => checkbox.checked = false);
        if(detailedInput) detailedInput.value = '';
    }

    function buttonState() {
        const feedbackPage = cookieUserData.pages[currentUrl];
        dislikeIcon.classList.remove('active');
        likeIcon.classList.remove('active');

        if(feedbackPage) {
            if(feedbackPage.result === true) {
                likeIcon.classList.add('active');
            } else if(feedbackPage.result === false) {
                dislikeIcon.classList.add('active');
            }
        }

        setCookie(cookieName, cookieUserData, 365);
    }

    async function sendFeedback(result, reasons = []) {
        const lastFeedback = cookieUserData.pages[currentUrl];
        if(lastFeedback) {
            const blockingFeedback = 5 * 60 * 1000;
            const timeSinceLastFeedback = Date.now() - lastFeedback.presentTime;
            if(timeSinceLastFeedback < blockingFeedback) {
                alert('Вы уже оставляли обратную связь, попробуйте позже');
                return;
            }
        }

        const jsonReasons = JSON.stringify(reasons);

        try {
            const feedbackData = {
                feedback_url: currentUrl,
                cookieUserId: cookieUserData.cookieUserId,
                result: result,
                reasons: jsonReasons
            };
            
            let url = 'https://deckhouse.ru/wp-json/articles-feedback/v1/feedback';
            // // let url = '/wp-json/articles-feedback/v1/feedback';
            url = url + '?user_ip=' + cookieUserData.cookieUserIp + '&uuid=' + feedbackData.cookieUserId + '&feedback_url=' + feedbackData.feedback_url + '&feedback_data=' + feedbackData.reasons;
            
            const response = await fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json;charset=utf-8',
                    Accept: "application/json",
                }
            });

            if(!response.ok) {
                throw new Error(response.status);
            }

            cookieUserData.pages[currentUrl] = {
                result: result,
                presentTime: Date.now()
            };

            setCookie(cookieName, cookieUserData, 365);

            buttonState();
            showAccessModal();
        } catch(error) {
            buttonState();
        }
    }

    likeIcon.addEventListener('click', async function() {
        await sendFeedback(true, []);
    })

    dislikeIcon.addEventListener('click', function() {
        showFormModal();
    })

    closeBtn.forEach(btn => {
        btn.addEventListener('click', function(e) {
            e.preventDefault();
            accessModal.style.display = 'none';
            formModal.style.display = 'none';
        })
    })

    moreDetailed.addEventListener('click', async function(e) {
        e.preventDefault();
        const reasons = [];
        formModal.querySelectorAll('.checkbox:checked').forEach(checkbox => {
            reasons.push(checkbox.value);
        })
        const detailedReason = detailedInput.value.trim();
        if(detailedReason.length > 0) {
            reasons.push(detailedReason);
        }

        if(reasons.length === 0 && detailedReason === '') {
            return;
        }

        hideFormModal();
        await sendFeedback(false, reasons)
    })

    window.addEventListener('beforeunload', function() {
        this.clearTimeout(accessModalTimeout);
    })
})
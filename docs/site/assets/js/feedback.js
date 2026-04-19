document.addEventListener('DOMContentLoaded', function () {
    const likeIcon = document.querySelector('.icon__like');
    const dislikeIcon = document.querySelector('.icon__dislike');
    const allModal = document.querySelectorAll('.window__feedback');
    const accessModal = document.querySelector('.window__feedback--access');
    const laterModal = document.querySelector('.window__feedback--later');
    const errorModal = document.querySelector('.window__feedback--error');
    const formModal = document.querySelector('.window__feedback--form');
    const moreDetailed = formModal ? formModal.querySelector('.button') : null;
    const closeBtn = document.querySelectorAll('.modal-window__close-btn');
    const detailedInput = document.querySelector('.more-detailed');
    const tocSidebar = document.querySelector('.layout-sidebar__sidebar_right');
    const currentUrl = window.location.href;
    const cookieName = 'userFeedback';

    allModal.forEach(modal => {
        if (modal) {
            if (tocSidebar) {
                modal.style.right = '-300px';
            } else {
                modal.style.right = '5px';
            }
        }
    })

    let accessModalTimeout;
    let laterModalTimeout;
    let errorModalTimeout;

    async function getUserIp() {
        try {
            const res = await fetch('https://api.ipify.org?format=json');
            const data = await res.json();
            return data.ip;
        } catch (error) {
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

        for (let i = 0; i < cookieArray.length; i++) {
            let cookie = cookieArray[i];
            while (cookie.charAt(0) === ' ') {
                cookie = cookie.substring(1);
            }
            if (cookie.indexOf(cookieName) === 0) {
                try {
                    return JSON.parse(cookie.substring(cookieName.length, cookie.length));
                } catch (error) {
                    return null;
                }
            }
        }
        return null;
    }

    function generateUUID() {
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
            const r = Math.floor(Math.random() * 16);
            const v = c === 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
    }

    let cookieUserData;

    (async function initUserData() {
        cookieUserData = getCookie(cookieName);

        if (!cookieUserData) {
            cookieUserData = { cookieUserId: generateUUID(), cookieUserIp: null, pages: {} };
        }

        if (!cookieUserData.cookieUserId) {
            cookieUserData.cookieUserId = generateUUID();
        }

        if (!cookieUserData.cookieUserIp) {
            const ip = await getUserIp();
            cookieUserData.cookieUserIp = ip;
            setCookie(cookieName, cookieUserData, 365);
        }

        buttonState();
    })();

    function showAccessModal() {
        if (accessModal) {
            accessModal.style.display = 'flex';
            clearTimeout(accessModalTimeout);
            accessModalTimeout = setTimeout(hideAccessModal, 10000);
        }
    }

    function hideAccessModal() {
        if (accessModal) {
            accessModal.style.display = 'none';
        }
    }

    function showFormModal() {
        if (formModal) {
            formModal.style.display = 'flex';
        }
    }

    function hideFormModal() {
        if (formModal) {
            formModal.style.display = 'none';
            formModal.querySelectorAll('.checkbox').forEach(checkbox => checkbox.checked = false);
        }
        if (detailedInput) detailedInput.value = '';
    }

    function showLaterModal() {
        if (laterModal) {
            laterModal.style.display = 'flex';
            clearTimeout(laterModalTimeout);
            laterModalTimeout = setTimeout(hideLaterModal, 10000);
        }
    }

    function hideLaterModal() {
        if (laterModal) {
            laterModal.style.display = 'none';
        }
    }

    function showErrorModal() {
        if (errorModal) {
            errorModal.style.display = 'flex';
            clearTimeout(errorModalTimeout);
            errorModalTimeout = setTimeout(hideLaterModal, 10000);
        }
    }

    function buttonState() {
        const feedbackPage = cookieUserData.pages[currentUrl];
        if (dislikeIcon) dislikeIcon.classList.remove('active');
        if (likeIcon) likeIcon.classList.remove('active');

        if (feedbackPage) {
            if (feedbackPage.state === true && likeIcon) {
                likeIcon.classList.add('active');
            } else if (feedbackPage.state === false && dislikeIcon) {
                dislikeIcon.classList.add('active');
            }
        }

        setCookie(cookieName, cookieUserData, 365);
    }

    async function sendFeedback(state, reasons = [], comment = '') {
        const jsonReasons = JSON.stringify(reasons);

        try {
            const feedbackData = {
                feedback_url: currentUrl,
                cookieUserId: cookieUserData.cookieUserId,
                result: state,
                reasons: jsonReasons,
                comment: comment
            };

            let url = '/wp-json/articles-feedback/v1/feedback';
            url = url + '?user_ip=' + cookieUserData.cookieUserIp + '&uuid=' + feedbackData.cookieUserId + '&feedback_url=' + feedbackData.feedback_url + '&feedback_data=' + feedbackData.reasons + '&feedback_comment=' + feedbackData.comment;
            const response = await fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json;charset=utf-8',
                    Accept: "application/json",
                }
            });

            if (!response.ok) {
                return response.text().then(text => {
                    throw new Error(response.status, text);
                })
            }

            cookieUserData.pages[currentUrl] = {
                state: state,
                presentTime: Date.now()
            };

            setCookie(cookieName, cookieUserData, 365);

            buttonState();
            showAccessModal();
        } catch (error) {
            buttonState();
            showErrorModal();
        }
    }

    if (likeIcon) {
        likeIcon.addEventListener('click', async function () {
            const lastFeedback = cookieUserData.pages[currentUrl];
            if(lastFeedback) {
                const blockingFeedback = 5 * 60 * 1000;
                const timeSinceLastFeedback = Date.now() - lastFeedback.presentTime;
                if(timeSinceLastFeedback < blockingFeedback) {
                    hideAccessModal();
                    showLaterModal();
                } else {
                    await sendFeedback(true, []);
                }
            } else  {
                await sendFeedback(true, [], '');
            }
        })
    }

    if (dislikeIcon) {
        dislikeIcon.addEventListener('click', function () {
            const lastFeedback = cookieUserData.pages[currentUrl];
            if(lastFeedback) {
                const blockingFeedback = 5 * 60 * 1000;
                const timeSinceLastFeedback = Date.now() - lastFeedback.presentTime;
                if(timeSinceLastFeedback < blockingFeedback) {
                    hideAccessModal();
                    showLaterModal();
                } else {
                    showFormModal();
                }
            } else {
                showFormModal();
            }
        })
    }

    if (moreDetailed && formModal) {
        moreDetailed.addEventListener('click', async function (e) {
            e.preventDefault();
            const reasons = [];
            let comment = '';

            formModal.querySelectorAll('.checkbox:checked').forEach(checkbox => {
                reasons.push(checkbox.value);
            })
            const detailedReason = detailedInput ? detailedInput.value.trim() : '';
            comment = detailedReason;

            if (reasons.length === 0 && detailedReason === '') {
                return;
            }
            hideFormModal();
            await sendFeedback(false, reasons, comment)
        })
    }

    closeBtn.forEach(btn => {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            if (accessModal) accessModal.style.display = 'none';
            if (formModal) formModal.style.display = 'none';
            if (errorModal) errorModal.style.display = 'none';
            if (laterModal) laterModal.style.display = 'none';
        })
    })

    window.addEventListener('beforeunload', function () {
        this.clearTimeout(accessModalTimeout);
    })
})

document.addEventListener('DOMContentLoaded', () => {
    const el = document.getElementById('feedback-app');
    if (!el) return;

    const { createApp, ref, computed, onMounted, onBeforeUnmount } = Vue;

    const app = createApp({
        setup() {
            const currentUrl = window.location.href;
            const cookieName = 'userFeedback';

            const activeModal = ref(null); // 'access', 'later', 'error', 'form'
            const formReasons = ref([]);
            const formDetailed = ref('');
            let cookieUserData = null;
            let modalTimeout = null;

            const isLiked = ref(false);
            const isDisliked = ref(false);

            // Dynamically apply right position depending on sidebar presence
            const modalStyle = computed(() => {
                const hasToc = document.querySelector('.layout-sidebar__sidebar_right');
                return {
                    display: 'flex', // overrides CSS display: none from original stylesheet
                    right: hasToc ? '-300px' : '5px'
                };
            });

            function getCookie(name) {
                const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
                if (match) {
                    try {
                        return JSON.parse(decodeURIComponent(match[2]));
                    } catch (e) {
                        return null;
                    }
                }
                return null;
            }

            function setCookie(name, value, days) {
                const date = new Date();
                date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
                document.cookie = name + '=' + encodeURIComponent(JSON.stringify(value)) + ';expires=' + date.toUTCString() + ';path=/';
            }

            function generateUUID() {
                return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
                    const r = Math.floor(Math.random() * 16);
                    const v = c === 'x' ? r : (r & 0x3 | 0x8);
                    return v.toString(16);
                });
            }

            async function initUserData() {
                cookieUserData = getCookie(cookieName) || { cookieUserId: generateUUID(), cookieUserIp: null, pages: {} };

                if (!cookieUserData.cookieUserId) cookieUserData.cookieUserId = generateUUID();

                if (!cookieUserData.cookieUserIp) {
                    try {
                        const res = await fetch('https://api.ipify.org?format=json');
                        const data = await res.json();
                        cookieUserData.cookieUserIp = data.ip;
                        setCookie(cookieName, cookieUserData, 365);
                    } catch (e) {
                        // ignore error, IP will be null
                    }
                }

                updateButtonState();
            }

            function updateButtonState() {
                const feedbackPage = cookieUserData.pages[currentUrl];
                isLiked.value = false;
                isDisliked.value = false;
                if (feedbackPage) {
                    if (feedbackPage.state === true) isLiked.value = true;
                    if (feedbackPage.state === false) isDisliked.value = true;
                }
                setCookie(cookieName, cookieUserData, 365);
            }

            function openModal(name, timeoutMs = null) {
                activeModal.value = name;
                if (modalTimeout) clearTimeout(modalTimeout);
                if (timeoutMs) {
                    modalTimeout = setTimeout(() => {
                        activeModal.value = null;
                    }, timeoutMs);
                }
            }

            function closeModal() {
                activeModal.value = null;
                if (modalTimeout) clearTimeout(modalTimeout);
            }

            function isSpamming() {
                const lastFeedback = cookieUserData.pages[currentUrl];
                if (lastFeedback) {
                    const timeSinceLastFeedback = Date.now() - lastFeedback.presentTime;
                    return timeSinceLastFeedback < (5 * 60 * 1000);
                }
                return false;
            }

            async function sendFeedback(state, reasons = [], comment = '') {
                try {
                    const payload = {
                        domain: window.location.hostname,
                        type: 'review-form',
                        user_ip: cookieUserData.cookieUserIp || '',
                        uuid: cookieUserData.cookieUserId,
                        result: state,
                        feedback_url: currentUrl,
                        feedback_data: reasons, // array of strings
                        feedback_comment: comment
                    };
                    
                    const url = 'https://forms.flant.ru/api/v1/form-submissions/send';
                    
                    const response = await fetch(url, {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                            Accept: "application/json",
                        },
                        body: JSON.stringify(payload)
                    });

                    if (!response.ok) throw new Error('API Error');

                    cookieUserData.pages[currentUrl] = { state, presentTime: Date.now() };
                    updateButtonState();
                    openModal('access', 10000);

                } catch (error) {
                    updateButtonState();
                    openModal('error', 10000);
                }
            }

            async function handleLike() {
                if (isSpamming()) {
                    openModal('later', 10000);
                } else {
                    await sendFeedback(true, [], '');
                }
            }

            function handleDislike() {
                if (isSpamming()) {
                    openModal('later', 10000);
                } else {
                    // Reset form fields
                    formReasons.value = [];
                    formDetailed.value = '';
                    openModal('form');
                }
            }

            async function submitForm() {
                if (formReasons.value.length === 0 && formDetailed.value.trim() === '') {
                    return;
                }
                closeModal();
                await sendFeedback(false, formReasons.value, formDetailed.value.trim());
            }

            onMounted(() => {
                initUserData();
            });

            onBeforeUnmount(() => {
                if (modalTimeout) clearTimeout(modalTimeout);
            });

            return {
                isLiked,
                isDisliked,
                activeModal,
                modalStyle,
                formReasons,
                formDetailed,
                handleLike,
                handleDislike,
                submitForm,
                closeModal
            };
        }
    });

    app.mount('#feedback-app');
});

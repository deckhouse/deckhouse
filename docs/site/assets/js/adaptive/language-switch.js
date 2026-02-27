document.addEventListener('DOMContentLoaded', function () {
    if (window.innerWidth >= 1024) return;
    $('#language-switch').each(function() {
        let pageDomain = window.location.hostname;
        if (window.location.pathname.startsWith('/ru/')) {
        $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('/ru/', '/en/')}'`)
        $(this).attr('checked', 'checked');
        } else if (window.location.pathname.startsWith('/en/')) {
        $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('/en/', '/ru/')}'`)
        $(this).removeAttr('checked', 'checked');
        } else {
            switch (pageDomain) {
                case 'deckhouse.io':
                $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('deckhouse.io', 'deckhouse.ru')}'`)
                $(this).removeAttr('checked', 'checked');
                break;
                case 'deckhouse.ru':
                $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('deckhouse.ru', 'deckhouse.io')}'`)
                $(this).attr('checked', 'checked');
                break;
                case 'ru.localhost':
                $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('ru.localhost', 'localhost')}'`)
                $(this).attr('checked', 'checked');
                break;
                case 'localhost':
                $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('localhost', 'ru.localhost')}'`)
                $(this).removeAttr('checked', 'checked');
                break;
                default:
                if (pageDomain.includes('deckhouse.ru.')) {
                    $(this).attr('onclick', `javascript:location.href='${ window.location.href.replace('deckhouse.ru.', 'deckhouse.')}'`)
                    $(this).attr('checked', 'checked');
                } else if (pageDomain.includes('deckhouse.')) {
                    $(this).attr('onclick', `javascript:location.href='${window.location.href.replace('deckhouse.', 'deckhouse.ru.')}'`)
                    $(this).removeAttr('checked', 'checked');
                }
            }
        }
    });
});

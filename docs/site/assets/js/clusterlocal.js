document.addEventListener("DOMContentLoaded", function () {
    $.getJSON('/config/data.json', {_: new Date().getTime()}).done(function (resp) {
        // data.json example: {"channel":"stable", "version":"xxxx", "edition":"EE"}

        let deckhouseVersionInfo = "";
        let update_channels_list = ['alpha', 'beta', 'early-access', 'stable', 'rock-solid'];

        if (resp) {
            if (resp['channel']) {
                $(`.releases__menu-item.releases__menu--channel--${resp['channel']}`).addClass("active");
                $(`.releases__menu-item-title.releases__menu--channel--${resp['channel']}`).addClass("active");
            } else {
                $(`div#releases__stale__block`).addClass("active");
                $(`div#releases__mark_note`).css("display", "none");
                console.log('UpdateChannel is not defined.');
            }

            if (resp['version'] || resp['edition'] || resp['channel']) {
                deckhouseVersionInfo = `${resp['version'] || 'unknown'}${resp['edition'] ? ` ${resp['edition']}` : ''}`;
            }

            $("#doc-versions-menu").append(`<span>${deckhouseVersionInfo}</span>`);
        } else {
            console.log('data.json is empty or not found.');
        }
    });
});

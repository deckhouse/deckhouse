document.addEventListener("DOMContentLoaded", function () {
    $.getJSON('/config/data.json', {_: new Date().getTime()}).done(function (resp) {
        // data.json example: {"channel":"stable", "version":"xxxx", "edition":"EE"}

        let deckhouseVersionInfo = "";
        let update_channels_list = ['alpha', 'beta', 'early-access', 'stable', 'rock-solid'];

        if (resp) {
            if (resp['channel']) {
                $(`.releases__menu-item.releases__menu--channel--${resp['channel']}`).addClass("releases__menu-item-block-active");
                $(`.releases__menu-item-title.releases__menu--channel--${resp['channel']}`).addClass("releases__menu-item-title-active");
            } else {
                console.log('UpdateChannel is not defined.');
            }

            if (resp['version']) {
                deckhouseVersionInfo = `${resp['version']}`;
            }

            if (resp['edition']) {
                deckhouseVersionInfo = `[${resp['edition']}] ${deckhouseVersionInfo}`;
            }

            $(".updatechannel__content").text(deckhouseVersionInfo).removeClass("disable");
        }
    });
});

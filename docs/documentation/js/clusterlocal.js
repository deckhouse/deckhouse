document.addEventListener("DOMContentLoaded", function () {
    $.getJSON('/config/data.json', {_: new Date().getTime()}).done(function (resp) {
        // data.json example: {"channel":"stable", "version":"xxxx", "edition":"FE"}

        let deckhouseVersionInfo = "Unknown";
        let update_channels_list = ['alpha', 'beta', 'early-access', 'stable', 'rock-solid'];

        if (resp) {
            if (resp['channel']) {
                $(`.releases__menu-item.releases__menu--channel--${resp['channel']}`).addClass("releases__menu-item-block-active");
                $(`.releases__menu-item-title.releases__menu--channel--${resp['channel']}`).addClass("releases__menu-item-title-active");
                if (update_channels_list.indexOf(resp['channel']) < 0) {
                    let isRuLang = document.location.pathname.match(/^\/ru\//);
                    if (isRuLang) {
                        deckhouseVersionInfo = `на ветке '${resp['channel']}'`;
                    } else {
                        deckhouseVersionInfo = `on the '${resp['channel']}' branch`;
                    }
                    $("#releases__stale__block").css({display: "block"});
                } else {
                    deckhouseVersionInfo = resp['channel'];
                    $("#releases__stale__block").css({display: "none"});
                }
            } else {
                console.log('UpdateChannel is not defined.');
            }

            if (resp['version'] && resp['version'] !== "dev") {
                deckhouseVersionInfo = `${resp['version']} (${deckhouseVersionInfo})`;
            }

            if (resp['edition']) {
                deckhouseVersionInfo = `[${resp['edition']}] ${deckhouseVersionInfo}`;
            }

            $(".updatechannel__content").text(deckhouseVersionInfo).removeClass("disable");
        }
    });
});

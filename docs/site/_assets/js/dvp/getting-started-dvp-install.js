document.addEventListener("DOMContentLoaded", function() {
    config_highlight();
    /* config_update + dvp после codeblock.js в footer, иначе baseline подсветки без номеров строк и после textContent в update_parameter. */
    setTimeout(function () {
        config_update();
        if (typeof dvp_config_update === 'function') {
            dvp_config_update();
        }
    }, 0);
});

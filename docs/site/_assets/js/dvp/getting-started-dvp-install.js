document.addEventListener("DOMContentLoaded", function() {
    config_highlight();
    config_update();
    if (typeof dvp_config_update === 'function') {
        dvp_config_update();
    }
});

function domain_update() {
    let domainPattern = sessionStorage.getItem('dhctl-domain');
    let domainSuffix = domainPattern ? domainPattern.replace('%s\.', '') : null;

    if (domainSuffix && domainPattern && domainSuffix.length > 0) {
        // Update rendered code block
        $('code span').filter(function () {
            return ((this.innerText.match(/[\S]+\.example\.com/i) || []).length > 0);
        }).each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].innerText : null;
            if (content && content.length > 0) {
                $(this)[0].innerText = content.replace(/([\S]+)\.example\.com/i, domainPattern.replace('%s', content.match(/([\S]+)\.example\.com/i)[1]));
            }
        });

        // Updating snippet
        $('[example-hosts]').each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].textContent : null;
            if (content && content.length > 0) {
                content.match(/([\S]+)\.example\.com/ig).forEach(function (item, index, arr) {
                    let serviceDomain = item.match(/([\S]+)\.example\.com/i)[1];
                    content = content.replace(/[\S]+.example\.com/i, domainPattern.replace('%s', serviceDomain));
                });
                $(this)[0].textContent = content;
            }
        });
    }
}

document.addEventListener("DOMContentLoaded", function() {
    domain_update();
});

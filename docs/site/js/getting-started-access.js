function domain_update() {
    var exampleDomainName = '%s\\.example\\.com'
    var exampleDomainSuffix = 'example\\.com';
    var domainPattern = sessionStorage.getItem('dhctl-domain');
    var domainSuffix = domainPattern ? domainPattern.replace('%s\.','') : null;

    if ( domainSuffix && domainSuffix.length > 0 ) {
        $('code span').filter(function () {
            return ((this.innerText.match(exampleDomainSuffix) || []).length > 0);
        }).each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].innerText : null;
            if (content && content.length > 0) {
                let re = new RegExp(exampleDomainSuffix, "g");
                $(this)[0].innerText = content.replace(re, domainSuffix);
            }
        });
    }
    update_parameter(domainSuffix, '', exampleDomainSuffix, null ,'[example-hosts]');
}

$( document ).ready(function() {
    domain_update();
});

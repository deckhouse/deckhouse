function domain_update() {
    const exampleDomainName = /%s\.example\.com/ig
    const exampleDomainSuffix = /example\.com/ig;
    var domainPattern = sessionStorage.getItem('dhctl-domain');
    var domainSuffix = domainPattern ? domainPattern.replace('%s\.',''): null ;

    if ( domainSuffix && domainSuffix.length > 0 ) {
        $('code').filter(function () {
            return ((this.innerText.match(exampleDomainSuffix) || []).length > 0);
        }).each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].innerText : null;
            if (content && content.length > 0) {
                let re = new RegExp(exampleDomainSuffix, "g");
                $(this)[0].innerText = content.replace(re, domainSuffix);
            }
        });

        $('a').filter(function () {
            return ((this.textContent.match(exampleDomainSuffix) || []).length > 0);
        }).each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].textContent : null;
            if (content && content.length > 0) {
                let re = new RegExp(exampleDomainSuffix, "g");
                $(this).attr('href', $(this).attr('href').replace(re, domainSuffix));
                $(this)[0].textContent = content.replace(re, domainSuffix);

            }
        });



    }
}

$( document ).ready(function() {
    domain_update();
    generate_password();
    replace_snippet_password();
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>',  null ,'code');
});

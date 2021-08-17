$(document).ready(function() {
    $('[gs-revision-tabs]').on('click', function() {
        var name = $(this).attr('data-features-tabs-trigger');
        var $parent = $(this).closest('[data-features-tabs]');
        var $triggers = $parent.find('[data-features-tabs-trigger]');
        var $contents = $parent.find('[data-features-tabs-content]');
        var $content = $parent.find('[data-features-tabs-content=' + name + ']');

        $triggers.removeClass('active');
        $contents.removeClass('active');

        $(this).addClass('active');
        $content.addClass('active');
    })

   set_license_token_cookie();

});

function config_highlight() {
    let matchMustChange = '!CHANGE_';
    let matchMightChangeEN = "# you might consider changing this";
    let matchMightChangeRU = "# возможно, захотите изменить";

    $('code span.c1').filter(function () {
        return (this.innerText === matchMightChangeEN) || (this.innerText === matchMightChangeRU);
    }).each(function (index) {
        // console.log($(this).next().id, '->' , $(this).next().innerText, $(this).next().textContent);
        // console.log($(this)[0].innerText, ' ->', $(this).next().next().next().text());
        try {
            if ($(this).next().next().next() && $(this).next().next().next().text() === '-') {
                $(this).next().next().next().next().addClass('mightChange');
            } else {
                $(this).next().next().next().addClass('mightChange');
            }
        } catch (e) {
            $(this).next().next().next().addClass('mightChange');
        }
        $(this).addClass('mightChange');
    });

    $('.language-yaml code span').filter(function () {
        result = this.innerText.match("!CHANGE_") ? this.innerText.match("!CHANGE_").length > 0 : false;
        return result;
    }).each(function (index) {
        $(this).prev().addClass('mustChange');
        $(this).addClass('mustChange');
    });
}

function config_update() {
   update_parameter('dhctl-prefix', 'prefix', 'cloud-demo', null ,'[config-yml]');
   update_parameter('dhctl-sshkey', 'sshPublicKey', 'ssh-rsa <SSH_PUBLIC_KEY>',  null ,'[config-yml]');
   update_parameter('dhctl-sshkey', 'sshKey', 'ssh-rsa <SSH_PUBLIC_KEY>',  null ,'[config-yml]');
   update_parameter('dhctl-layout', 'layout', '<layout>',  null ,'[config-yml]');
   let preset = sessionStorage.getItem('dhctl-preset');
   if ( preset && preset.length > 0 ) {
      if ( ['production','ha'].includes(preset)) {
          update_parameter('dhctl-preset', 'replicas', '1', 3  );
          update_parameter('dhctl-preset', '', 'replicas: 1', 'replicas: 3',  '[config-yml]');
          if ($('#platform_code') && $('#platform_code').text() === 'yandex') {
              magic4yandex();
          }
      }
   }
   update_license_parameters();
   update_domain_parameters();
}

function update_domain_parameters() {
   const exampleDomainName = /%s\.example\.com/ig
   const exampleDomainSuffix = /example\.com/ig;
   let dhctlDomain = sessionStorage.getItem('dhctl-domain')
   let dhctlDomainSuffx = dhctlDomain ? sessionStorage.getItem('dhctl-domain').replace('%s\.','') : null;

   // update_parameter('dhctl-domain', 'publicDomainTemplate', exampleDomainName, null ,'[config-yml]');
   // modify snippet content
    $('[config-yml]').each(function (index) {
        let content = ($(this)[0]) ? $(this)[0].textContent : null ;
        if (dhctlDomainSuffx && content && content.length > 0) {
            let re = new RegExp(exampleDomainSuffix, "g");
            $(this)[0].textContent = content.replace(re, dhctlDomainSuffx);
        }
    });
    // modify codeblock
    $('code span').filter(function () {
        return ( (this.innerText.match(exampleDomainSuffix) || []).length > 0 ) ;
    }).each(function (index) {
        let content = ($(this)[0]) ? $(this)[0].textContent : null ;
        if (dhctlDomainSuffx && content && content.length > 0) {
            let re = new RegExp(exampleDomainSuffix, "g");
            $(this)[0].textContent = content.replace(re, dhctlDomainSuffx);
        }
    });

   update_parameter((sessionStorage.getItem('dhctl-domain')||'example.com').replace('%s.',''), null, 'example.com',  null ,'[resources-yml]');
}

function update_parameter(sourceDataName, searchKey, replacePattern, value = null, snippetSelector= '' ) {
    var objectToModify, sourceData;

    if ( sourceDataName && sourceDataName.match(/^dhctl-/) ) {
        sourceData = sessionStorage.getItem(sourceDataName);
    } else {
        sourceData = sourceDataName;
    }

    if (sourceData && sourceData.length > 0) {
        if (searchKey && searchKey.length > 0) {
            $('code span').filter(function () {
                return this.innerText === searchKey;
            }).each(function (index) {
                if ($(this).next().next()[0] && $(this).next().next()[0].innerText === '"') {
                    objectToModify = $(this).next().next().next()[0]
                } else {
                    objectToModify = $(this).next().next()[0]
                }
                if (objectToModify && (objectToModify.innerText.length > 0)) {
                    let innerText = objectToModify.innerText;
                    if ( replacePattern === '<GENERATED_PASSWORD_HASH>' ) {
                        objectToModify.innerText = innerText.replace(replacePattern, "'" + (value ? value : sourceData) + "'");
                    } else {
                        objectToModify.innerText = innerText.replace(replacePattern, value ? value : sourceData);
                    }
                }
            });
        }

        if (snippetSelector && snippetSelector.length > 0) {
            $(snippetSelector).each(function (index) {
                let content = ($(this)[0]) ? $(this)[0].textContent : null ;
                if (content && content.length > 0) {
                    let re = new RegExp(replacePattern, "g");
                    if ( replacePattern === '<GENERATED_PASSWORD_HASH>' ) {
                        $(this)[0].textContent = content.replace(re, "'" + (value ? value : sourceData) + "'");
                    } else {
                        $(this)[0].textContent = content.replace(re, value ? value : sourceData);
                    }
                }
            });
        }
    }
}

// Update license token and docker config
function update_license_parameters() {
    if ($.cookie("demotoken") || $.cookie("license-token") ) {
        let username = 'license-token';
        let password = $.cookie("license-token") ? $.cookie("license-token") : $.cookie("demotoken");
        let registry = 'registry.deckhouse.io';
        let auth = btoa(username + ':' + password);
        let config = '{"auths": { "'+ registry +'": { "username": "'+ username +'", "password": "' + password + '", "auth": "' + auth +'"}}}';
        let matchStringClusterConfig = '<YOUR_ACCESS_STRING_IS_HERE>';
        let matchStringDockerLogin = "<LICENSE_TOKEN>";

       update_parameter(btoa(config), 'registryDockerCfg', matchStringClusterConfig, null ,'[config-yml]');
       update_parameter(password, '', matchStringDockerLogin, null ,'[docker-login]');

        // $('code span.s').filter(function () {
        //     return this.innerText == matchStringClusterConfig;
        // }).text(btoa(config));
        //
        $('.highlight code').filter(function () {
            return this.innerText.match(matchStringDockerLogin) == matchStringDockerLogin;
        }).each(function(index) {
            $(this).text($(this).text().replace(matchStringDockerLogin,password));
        });
    } else {
        console.log("No license token, so InitConfiguration was not updated");
    }
}

function generate_password() {
    if (sessionStorage.getItem("dhctl-user-password-hash") === null || sessionStorage.getItem("dhctl-user-password") === null) {
      var bcrypt = dcodeIO.bcrypt;
      var salt = bcrypt.genSaltSync(10);
      var password = Math.random().toString(36).slice(-10);
      var hash = bcrypt.hashSync(password, salt);
      sessionStorage.setItem("dhctl-user-password-hash", hash);
      sessionStorage.setItem("dhctl-user-password", password);
    }
}

function replace_snippet_password() {
   update_parameter('dhctl-user-password-hash', 'password', '<GENERATED_PASSWORD_HASH>',  null ,null);
   update_parameter('dhctl-user-password-hash', null, '<GENERATED_PASSWORD_HASH>',  null ,'[resources-yml]');
   update_parameter(    'dhctl-user-password', null, '<GENERATED_PASSWORD>',  null ,'[resources-yml]');
   update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>',  null ,'code span.c1');
   update_parameter('dhctl-domain', null, '<GENERATED_PASSWORD>',  null ,'code span.c1');
}

// Set license-token cookie if it pass in the license-token GET parameter
function set_license_token_cookie() {
    let urlParams = new URLSearchParams(window.location.search);
    if (urlParams.has('license-token')) {
        let token = urlParams.get('license-token');
        $.cookie('license-token', token, {path: '/' });
    }
}

function magic4yandex() {
    $('code span').filter(function () {
        return this.innerText === 'externalIPAddresses';
    }).each(function (index) {
        $(this).next().append("\n    <span class=\"pi\">-</span> <span class=\"s2\">\"</span><span class=\"s\">Auto\"</span>");
        $(this).next().append("\n    <span class=\"pi\">-</span> <span class=\"s2\">\"</span><span class=\"s\">Auto\"</span>");
    })

    $('[config-yml]').each(function (index) {
        let content = ($(this)[0]) ? $(this)[0].textContent : null ;
        if (content && content.length > 0) {
            $(this)[0].textContent = content.replace("    externalIPAddresses:\n    - \"Auto\"\n", "    externalIPAddresses:\n    - \"Auto\"\n    - \"Auto\"\n    - \"Auto\"\n");
        }
    });
}

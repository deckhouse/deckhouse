$(document).ready(function () {
    $('[gs-revision-tabs]').on('click', function () {
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
    update_parameter('dhctl-prefix', 'prefix', 'cloud-demo', null, '[config-yml]');
    update_parameter('dhctl-sshkey', 'sshPublicKey', 'ssh-rsa <SSH_PUBLIC_KEY>', null, '[config-yml]');
    update_parameter('dhctl-sshkey', 'sshKey', 'ssh-rsa <SSH_PUBLIC_KEY>', null, '[config-yml]');
    update_parameter('dhctl-layout', 'layout', '<layout>', null, '[config-yml]');
    let preset = sessionStorage.getItem('dhctl-preset');
    if (preset && preset.length > 0) {
        if (['production', 'ha'].includes(preset)) {
            update_parameter('dhctl-preset', 'replicas', '1', 3);
            update_parameter('dhctl-preset', '', 'replicas: 1', 'replicas: 3', '[config-yml]');
            if ($('#platform_code') && $('#platform_code').text() === 'yandex') {
                magic4yandex();
            }
        }
    }
    update_license_parameters();
    update_domain_parameters();
}

function update_domain_parameters() {
    let dhctlDomain = sessionStorage.getItem('dhctl-domain');

    update_parameter('dhctl-domain', 'publicDomainTemplate', '%s.example.com', null, '[config-yml]');
    // update domain template example in code block
    $('code span').filter(function () {
        return ((this.innerText.match('%s.example.com') || []).length > 0);
    }).each(function (index) {
        let content = ($(this)[0]) ? $(this)[0].innerText : null;
        if (content && content.length > 0 && dhctlDomain) {
            $(this)[0].innerText = content.replace('%s.example.com', dhctlDomain).replace('grafana.example.com', dhctlDomain.replace('%s', 'grafana'));
        }
    });

    // update domain template example in snippet
    $('[config-yml]').each(function (index) {
        let content = ($(this)[0]) ? $(this)[0].textContent : null;
        if (content && content.length > 0 && dhctlDomain) {
            $(this)[0].textContent = content.replace('grafana.example.com', dhctlDomain.replace('%s', 'grafana'));
        }
    });

    // update user email
    $('code span').filter(function () {
        return ((this.innerText.match('admin@example.com') || []).length > 0);
    }).each(function (index) {
        let content = ($(this)[0]) ? $(this)[0].innerText : null;
        if (content && content.length > 0 && dhctlDomain) {
            $(this)[0].innerText = content.replace('admin@example.com', 'admin@' + dhctlDomain.replace(/%s[^.]*./, ''));
        }
    });
    // update user email in the resources-yml or user-yml snippet
    $('[resources-yml],[user-yml]').each(function (index) {
        let content = ($(this)[0]) ? $(this)[0].textContent : null;
        if (content && content.length > 0 && dhctlDomain) {
            $(this)[0].textContent = content.replace(/admin@example.com/g, 'admin@' + dhctlDomain.replace(/%s[^.]*./, ''));
        }
    });

    update_parameter((sessionStorage.getItem('dhctl-domain') || 'example.com').replace('%s.', ''), null, 'example.com', null, '[resources-yml]');
}

function update_parameter(sourceDataName, searchKey, replacePattern, value = null, snippetSelector = '', multilineIndent = 0) {
    var objectToModify, sourceData;

    if (sourceDataName && sourceDataName.match(/^dhctl-/)) {
        sourceData = sessionStorage.getItem(sourceDataName);
    } else {
        sourceData = sourceDataName;
    }

    if (multilineIndent > 0) {
        value = (value ? value : sourceData).replace(/^/gm, ' '.repeat(multilineIndent));
        value = "|\n" + value;
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
                    if (replacePattern === '<GENERATED_PASSWORD_HASH>') {
                        objectToModify.innerText = innerText.replace(replacePattern, "'" + (value ? value : sourceData) + "'");
                    } else {
                        objectToModify.innerText = innerText.replace(replacePattern, value ? value : sourceData);
                    }
                }
            });
        }

        if (snippetSelector && snippetSelector.length > 0) {
            $(snippetSelector).each(function (index) {
                let content = ($(this)[0]) ? $(this)[0].textContent : null;
                if (content && content.length > 0) {
                    let re = new RegExp(replacePattern, "g");
                    if (replacePattern === '<GENERATED_PASSWORD_HASH>') {
                        $(this)[0].textContent = content.replace(re, "'" + (value ? value : sourceData) + "'");
                    } else {
                        $(this)[0].textContent = content.replace(re, value ? value : sourceData);
                    }
                }
            });
        }
    }
}

function updateTextInSnippet(snippetSelector, replacePattern, value) {
  $(snippetSelector).each(function (index) {
    let content = ($(this)[0]) ? $(this)[0].textContent : null;
    if (content && content.length > 0) {
      this.textContent = content.replace(replacePattern, value);
    }
  });
}

function getDockerAuthFromToken(username, password) {
    return btoa(username + ':' + password);
}

function getDockerConfigFromToken(registry, username, password) {
    return btoa('{"auths": { "' + registry + '": { "username": "' + username + '", "password": "' + password + '", "auth": "' + getDockerAuthFromToken(username, password) + '"}}}');
}

// Update license token and docker config
function update_license_parameters(newtoken = '') {
    if ($.cookie("demotoken") || $.cookie("license-token") || newtoken !== '') {
        let registry = 'registry.deckhouse.io';
        let username = 'license-token';
        let matchStringClusterConfig = '<YOUR_ACCESS_STRING_IS_HERE>';
        let matchStringDockerLogin = 'echo <LICENSE_TOKEN>';
        let password = $.cookie("license-token") ? $.cookie("license-token") : $.cookie("demotoken");
        let passwordHash = btoa(password);

        if (newtoken) {
            if ( password ) {
                matchStringClusterConfig = getDockerConfigFromToken(registry, username, password);
                matchStringDockerLogin = 'base64 -d <<< ' + passwordHash;
            }
            password = newtoken;
            passwordHash = btoa(password);
            $.cookie('license-token', newtoken, {path: '/' ,  expires: 365 })
        }

        let config = getDockerConfigFromToken(registry, username, password);
        let replacePartStringDockerLogin = 'base64 -d <<< ' + passwordHash;

        update_parameter(config, 'registryDockerCfg', matchStringClusterConfig, null, '[config-yml]');
        update_parameter(replacePartStringDockerLogin , '', matchStringDockerLogin, null, '[docker-login]');
        $('.highlight code').filter(function () {
            return this.innerText.match(matchStringDockerLogin) == matchStringDockerLogin;
        }).each(function (index) {
            $(this).text($(this).text().replace(matchStringDockerLogin, replacePartStringDockerLogin));
        });
    } else {
        console.log("No license token, so InitConfiguration was not updated");
    }
}

function generate_password(force = false) {
    if ( force || sessionStorage.getItem("dhctl-user-password-hash") === null || sessionStorage.getItem("dhctl-user-password") === null) {
        var bcrypt = dcodeIO.bcrypt;
        var salt = bcrypt.genSaltSync(10);
        var password = Math.random().toString(36).slice(-10);
        var hash = bcrypt.hashSync(password, salt);
        sessionStorage.setItem("dhctl-user-password-hash", hash);
        sessionStorage.setItem("dhctl-user-password", password);
    }
}

function replace_snippet_password() {
    update_parameter('dhctl-user-password-hash', 'password', '<GENERATED_PASSWORD_HASH>', null, null);
    update_parameter('dhctl-user-password-hash', null, '<GENERATED_PASSWORD_HASH>', null, '[resources-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, '[resources-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, 'code span.c1');
    update_parameter('dhctl-domain', null, '<GENERATED_PASSWORD>', null, 'code span.c1');
}

// Set license-token cookie if it pass in the license-token GET parameter
function set_license_token_cookie() {
    let urlParams = new URLSearchParams(window.location.search);
    if (urlParams.has('license-token')) {
        let token = urlParams.get('license-token');
        $.cookie('license-token', token, {path: '/',  expires: 365 });
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
        let content = ($(this)[0]) ? $(this)[0].textContent : null;
        if (content && content.length > 0) {
            $(this)[0].textContent = content.replace("    externalIPAddresses:\n    - \"Auto\"\n", "    externalIPAddresses:\n    - \"Auto\"\n    - \"Auto\"\n    - \"Auto\"\n");
        }
    });
}

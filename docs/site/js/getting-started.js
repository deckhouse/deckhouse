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
  let matchMightChangeEN = /# [Yy]ou might consider changing this\.?/;
  let matchMightChangeRU = /# [Вв]озможно, захотите изменить\.?/;

  $('code span.c1').filter(function () {
    return (matchMightChangeEN.test(this.innerText)) || (matchMightChangeRU.test(this.innerText));
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
  update_parameter('dhctl-sshkey', 'sshPublicKey', '<SSH_PUBLIC_KEY>', null, '[config-yml]');
  update_parameter('dhctl-sshkey', 'sshKey', '<SSH_PUBLIC_KEY>', null, '[config-yml]');
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
    return ((this.innerText.match('admin@deckhouse.io') || []).length > 0);
  }).each(function (index) {
    let content = ($(this)[0]) ? $(this)[0].innerText : null;
    if (content && content.length > 0 && dhctlDomain) {
      $(this)[0].innerText = content.replace('admin@deckhouse.io', 'admin@' + dhctlDomain.replace(/%s[^.]*./, ''));
    }
  });
  // update user email in the resources-yml or user-yml snippet
  $('[resources-yml],[user-yml]').each(function (index) {
    let content = ($(this)[0]) ? $(this)[0].textContent : null;
    if (content && content.length > 0 && dhctlDomain) {
      $(this)[0].textContent = content.replace(/admin@deckhouse.io/g, 'admin@' + dhctlDomain.replace(/%s[^.]*./, ''));
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
    value = value ? value : sourceData
    if (value) {
      value = value.replace(/^/gm, ' '.repeat(multilineIndent));
      value = "|\n" + value;
    }
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

//
// Removes `disabled` class on target block selector if the item has a value otherwise, adds `disabled` class.
//
function triggerBlockOnItemContent(itemSelector, targetSelector, turnCommonElement = false) {
  const input = $(itemSelector);
  const wrapper = $(targetSelector);
  if (input.val() !== '') {
    update_license_parameters(input.val().trim());
    wrapper.removeClass('disabled');
  } else if(input.val() === '' && !turnCommonElement) {
    getLicenseToken(input.val());
  } else {
    wrapper.addClass('disabled');
    if (turnCommonElement) {
      $(targetSelector + '.common').removeClass('disabled');
      console.log('Turn common element');
    }
  }
}

function toggleDisabled(tab, inputDataAttr) {
  if (tab === 'tab_layout_ce' ) {
    $('.dimmer-block-content.common').removeClass('disabled');
  } else if (tab === 'tab_layout_ee' ) {
    const licenseToken = $(inputDataAttr).val().trim();
    getLicenseToken(licenseToken)
  }
}

async function getLicenseToken(token) {
  try {
    if (token === '') {
      throw new Error(responseFromLicense[pageLang]['empty_input']);
    }
    const span = $($('#enter-license-key').next('span'));
    const input = $('[license-token]');
    const response = await fetch(`https://license.deckhouse.io/api/license/check?token=${token}`);
    if(response.ok) {
      const data = await response.json();
      handlerResolveData(data, token, span, input);
    } else {
      handlerRejectData(token, span, input);
    }
  } catch (e) {
    const span = $($('#enter-license-key').next('span'));
    const input = $('[license-token]');
    handlerRejectData(token, span, input, e.message);
  }
}

function handlerResolveData(data, licenseToken, messageElement, inputField) {
  messageElement.html(`${responseFromLicense[pageLang]['resolve']}`);
  messageElement.removeAttr('class').addClass('license-form__message');

  $('.dimmer-block-content').removeClass('disabled');
  inputField.removeClass('license-token-input--error');
  inputField.addClass('license-token-input--success');

  update_license_parameters(licenseToken);
}

function handlerRejectData(licenseToken, messageElement, inputField, message = null) {
  if (message) {
    messageElement.html(message);
  } else {
    messageElement.html(responseFromLicense[pageLang]['reject']);
  }
  messageElement.removeAttr('class').addClass('license-form__warn');

  licenseToken = '';
  $.removeCookie('license-token', {path: '/'});

  update_license_parameters(licenseToken);
  $('.dimmer-block-content').addClass('disabled');
  $('.dimmer-block-content.common').addClass('disabled');
  inputField.removeClass('license-token-input--success');
  inputField.addClass('license-token-input--error');
}

// Update license token and docker config
function update_license_parameters(newtoken = '') {

  if ($.cookie("demotoken") || $.cookie("license-token") || newtoken !== '') {
    let registry = 'registry.deckhouse.io';
    if ($.cookie("lang") === "ru") {
      registry = 'registry.deckhouse.ru'
    }
    let username = 'license-token';
    let matchStringClusterConfig = '<YOUR_ACCESS_STRING_IS_HERE>';
    let matchStringDockerLogin = 'echo <LICENSE_TOKEN>';
    let password = $.cookie("license-token") ? $.cookie("license-token") : $.cookie("demotoken");
    let passwordHash = btoa(password);

    if (newtoken) {
      if (password) {
        matchStringClusterConfig = getDockerConfigFromToken(registry, username, password);
        matchStringDockerLogin = 'base64 -d <<< ' + passwordHash;
      }
      password = newtoken;
      passwordHash = btoa(password);
      $.cookie('license-token', newtoken, {path: '/', expires: 1})

    }

    let config = getDockerConfigFromToken(registry, username, password);
    let replacePartStringDockerLogin = 'base64 -d <<< ' + passwordHash;

    update_parameter(config, 'registryDockerCfg', matchStringClusterConfig, null, '[config-yml]');
    update_parameter(replacePartStringDockerLogin, '', matchStringDockerLogin, null, '[docker-run]');
    update_parameter(replacePartStringDockerLogin, '', matchStringDockerLogin, null, '[docker-run-windows]');
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
  if (force || sessionStorage.getItem("dhctl-user-password-hash") === null || sessionStorage.getItem("dhctl-user-password") === null) {
    var bcrypt = dcodeIO.bcrypt;
    var salt = bcrypt.genSaltSync(10);
    var password = Math.random().toString(36).slice(-10);
    var hash = bcrypt.hashSync(password, salt);
    var base64 = btoa(hash)
    sessionStorage.setItem("dhctl-user-password-hash", base64);
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
    $.cookie('license-token', token, {path: '/', expires: 365});
  }
}

$(document).ready(function () {
  let modulesProxyEnabled = sessionStorage.getItem('dhctl-modules-proxy-enabled')
  let modulesProxyHttpsUri = sessionStorage.getItem('dhctl-modules-proxy-https-uri')
  let modulesProxyHttpUri = sessionStorage.getItem('dhctl-modules-proxy-http-uri')
  let modulesNoProxyAddressList = sessionStorage.getItem('dhctl-modules-noproxy-address-list')
  let packagesProxyEnabled = sessionStorage.getItem('dhctl-packages-proxy-enabled')
  let packagesProxyUsername = sessionStorage.getItem('dhctl-packages-proxy-username')
  let packagesProxyPassword = sessionStorage.getItem('dhctl-packages-proxy-password')
  let packagesProxyURI = sessionStorage.getItem('dhctl-packages-proxy-uri')
  let registryDockerCfg = sessionStorage.getItem('dhctl-registry-docker-cfg')
  let registryImagesRepo = sessionStorage.getItem('dhctl-registry-images-repo')
  let registrySchemeHTTP = sessionStorage.getItem('dhctl-registry-scheme-http')
  let registryCA = sessionStorage.getItem('dhctl-registry-ca')

  if (modulesNoProxyAddressList && modulesNoProxyAddressList.length > 0) {
    modulesNoProxyAddressList = modulesNoProxyAddressList.replaceAll(' ', '')
  }

  if (modulesProxyEnabled && modulesProxyEnabled === "true" &&
    ((modulesProxyHttpsUri && modulesProxyHttpsUri.length > 0) || (modulesProxyHttpUri && modulesProxyHttpUri.length > 0))) {
    if (modulesProxyHttpsUri && modulesProxyHttpsUri.length > 0) {
      update_parameter('dhctl-modules-proxy-https-uri', 'httpsProxy', '<HTTPS_PROXY_ADDRESS>', null, '[config-yml]');
      if (!(modulesProxyHttpUri && modulesProxyHttpUri.length > 0)) {
        // Delete httpProxy section
        $('code span.na').filter(function () {
          return (this.innerText === "httpProxy");
        }).each(function (index) {
          parent = $(this).parent();
          delete_elements($(this), 2);
          parent.html(parent.html().replaceAll(/\n\s+\n/g, "\n"));
        });
        updateTextInSnippet('[config-yml]', /\n\s+httpProxy: <HTTP_PROXY_ADDRESS>\n/, "\n");
      }
    }

    if (modulesProxyHttpUri && modulesProxyHttpUri.length > 0) {
      update_parameter('dhctl-modules-proxy-http-uri', 'httpProxy', '<HTTP_PROXY_ADDRESS>', null, '[config-yml]');
      if (!(modulesProxyHttpsUri && modulesProxyHttpsUri.length > 0)) {
        // Delete httpsProxy section
        $('code span.na').filter(function () {
          return (this.innerText === "httpsProxy");
        }).each(function (index) {
          parent = $(this).parent();
          delete_elements($(this), 2);
          parent.html(parent.html().replaceAll(/\n\s+\n/g, "\n"));
        });
        updateTextInSnippet('[config-yml]', /\n\s+httpsProxy: <HTTPS_PROXY_ADDRESS>\n/, "\n");
      }
    }

    if (modulesNoProxyAddressList && modulesNoProxyAddressList.length > 0) {
      modulesNoProxyAddressList = ('["' + modulesNoProxyAddressList.split(',').join('", "') + '"]').replaceAll('""', '"')
      update_parameter(modulesNoProxyAddressList, 'noProxy', '<NO_PROXY_LIST>', null, '[config-yml]');
    }
  } else {
    // Delete proxy section
    $('code span.na').filter(function () {
      return (this.innerText === "proxy");
    }).each(function (index) {
      parent = $(this).parent();
      delete_elements($(this).prev(), 12);
      parent.html(parent.html().replaceAll(/\n\s+\n/g, "\n"));
    });
    updateTextInSnippet('[config-yml]', /\n\s+#[^#]+\n\s+proxy:\n\s+httpProxy: <HTTP_PROXY_ADDRESS>\n\s+httpsProxy: <HTTPS_PROXY_ADDRESS>\n\s+noProxy: <NO_PROXY_LIST>\n/, "\n");
  }

  if (!(modulesNoProxyAddressList && modulesNoProxyAddressList.length > 0)) {
    // Delete noProxy section
    $('code span.na').filter(function () {
      return (this.innerText === "noProxy");
    }).each(function (index) {
      parent = $(this).parent();
      delete_elements($(this), 2);
      parent.html(parent.html().replaceAll(/\n\s+\n/g, "\n"));
    });
    updateTextInSnippet('[config-yml]', /\n\s+noProxy: <NO_PROXY_LIST>\n/, "\n");
  }

  if (packagesProxyEnabled && packagesProxyEnabled === "true" &&
    packagesProxyUsername && packagesProxyUsername.length > 0 &&
    packagesProxyPassword && packagesProxyPassword.length > 0 &&
    packagesProxyURI && packagesProxyURI.length > 0) {
    update_parameter('dhctl-packages-proxy-uri', 'uri', 'https://example.com', null, '[config-yml]');
    update_parameter('dhctl-packages-proxy-username', 'username', '<PROXY-USERNAME>', null, '[config-yml]');
    update_parameter('dhctl-packages-proxy-password', 'password', '<PROXY-PASSWORD>', null, '[config-yml]');
  } else if (packagesProxyEnabled && packagesProxyEnabled === "true" && packagesProxyURI && packagesProxyURI.length > 0) {
    // Have proxy without auth.
    update_parameter('dhctl-packages-proxy-uri', 'uri', 'https://example.com', null, '[config-yml]');
    $('code span.na').filter(function () {
      return (this.innerText === "packagesProxy");
    }).each(function (index) {
      delete_elements($(this).next().next().next().next().next(), 5);
    });
    updateTextInSnippet('[config-yml]', /\s+username: <PROXY-USERNAME>\n\s+password: <PROXY-PASSWORD>\n---/, "\n---");
  } else {
    // Have no info to fill packagesProxy array, delete it from config
    console.log("Have no info to fill packagesProxy array...");
    $('code span.na').filter(function () {
      return (this.innerText === "packagesProxy");
    }).each(function (index) {
      delete_elements($(this), 10);
    });
    updateTextInSnippet('[config-yml]', /packagesProxy.+<PROXY-PASSWORD>\n---/s, "---");
  }

  if (registryImagesRepo && registryImagesRepo.length > 0) {
    // trim right / symbol
    // for example: registry.deckhouse.io/deckhouse/ce/ -> registry.deckhouse.io/deckhouse/ce
    const cleanedRegistryImagesRepo = registryImagesRepo.replace(/\/+$/, '');
    update_parameter('dhctl-registry-docker-cfg', 'registryDockerCfg', '<YOUR_PRIVATE_ACCESS_STRING_IS_HERE>', null, '[config-yml]');
    update_parameter('dhctl-registry-images-repo', 'imagesRepo', '<IMAGES_REPO_URI>', null, '[config-yml]');
    update_parameter('dhctl-registry-ca', 'registryCA', '<REGISTRY_CA>', null, '[config-yml]', 4);
    if (registrySchemeHTTP && registrySchemeHTTP === 'true') {
      update_parameter('HTTP', 'registryScheme', 'HTTPS', null, null);
      updateTextInSnippet('[config-yml]', /registryScheme: HTTPS.+\n---/s, "registryScheme: HTTP\n---");
    }

    if ((registrySchemeHTTP && registrySchemeHTTP === 'true') || !registryCA || (registryCA && registryCA.length < 1)) {
      // delete the registryCA parameter
      $('code span.na').filter(function () {
        return (this.innerText === "registryScheme");
      }).each(function (index) {
        delete_elements($(this).next().next().next(), 3);
        updateTextInSnippet('[config-yml]', /(registryScheme: HTTP[S]?).+\n---/s, "$1\n---");
      });

    }
    $('.highlight code').filter(function () {
      return this.innerText.match('<IMAGES_REPO_URI>');
    }).each(function () {
      $(this).text($(this).text().replace('<IMAGES_REPO_URI>', cleanedRegistryImagesRepo));
    });

    updateTextInSnippet('[docker-login-ce]', '<IMAGES_REPO_URI>', cleanedRegistryImagesRepo);
  }

  // delete empty lines in snippet
  $('div.snippetcut code').each(function () {
    let text = this.innerHTML;
    this.innerHTML = text.replace(/\s+\n(<span class="nn">---)/gs, "\n$1");
  });

});

// Deletes element and the next count elements
function delete_elements(element, count) {
  // console.log("input - ", element, "count - ", count)
  if (count && count > 0) {
    delete_elements(element.next(), count - 1)
  }
  // console.log("remove - ", element)
  element.remove()
}

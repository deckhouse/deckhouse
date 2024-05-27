$(document).ready(function () {
  let proxyHttpsURI = sessionStorage.getItem('dhctl-proxy-https-uri')
  let proxyHttpURI = sessionStorage.getItem('dhctl-proxy-http-uri')
  let noProxyAddressList = sessionStorage.getItem('dhctl-noproxy-address-list')
  let registryDockerCfg = sessionStorage.getItem('dhctl-registry-docker-cfg')
  let registryImagesRepo = sessionStorage.getItem('dhctl-registry-images-repo')
  let registrySchemeHTTP = sessionStorage.getItem('dhctl-registry-scheme-http')
  let registryCA = sessionStorage.getItem('dhctl-registry-ca')

  if (noProxyAddressList && noProxyAddressList.length > 0) {
    noProxyAddressList = noProxyAddressList.replaceAll(' ', '')
  }

  if ( (proxyHttpsURI && proxyHttpsURI.length > 0) || (proxyHttpURI && proxyHttpURI.length > 0) ) {
    if (proxyHttpsURI && proxyHttpsURI.length > 0) {
      update_parameter('dhctl-proxy-https-uri', 'httpsProxy', '<HTTPS_PROXY_ADDRESS>', null, '[config-yml]');
      if (!(proxyHttpURI && proxyHttpURI.length > 0)) {
        // Delete httpProxy section
        $('code span.na').filter(function () {
          return (this.innerText === "httpProxy");
        }).each(function (index) {
          parent = $(this).parent();
          delete_elements($(this), 2);
          parent.html(parent.html().replaceAll(/\n\s+\n/g, "\n"));
        });
        updateTextInSnippet('[config-yml]', /\n\s+httpProxy: \<HTTP_PROXY_ADDRESS\>\n/, "\n");
      }
    }

    if (proxyHttpURI && proxyHttpURI.length > 0) {
      update_parameter('dhctl-proxy-http-uri', 'httpProxy', '<HTTP_PROXY_ADDRESS>', null, '[config-yml]');
      if (!(proxyHttpsURI && proxyHttpsURI.length > 0)) {
        // Delete httpsProxy section
        $('code span.na').filter(function () {
          return (this.innerText === "httpsProxy");
        }).each(function (index) {
          parent = $(this).parent();
          delete_elements($(this), 2);
          parent.html(parent.html().replaceAll(/\n\s+\n/g, "\n"));
        });
        updateTextInSnippet('[config-yml]', /\n\s+httpsProxy: \<HTTPS_PROXY_ADDRESS\>\n/, "\n");
      }
    }

    if ((noProxyAddressList && noProxyAddressList.length > 0) || !(noProxyAddressList && noProxyAddressList.length > 0)) {
      noProxyAddressList = ('["' + noProxyAddressList.split(',').join('", "') + '"]').replaceAll('""', '"')
      update_parameter(noProxyAddressList, 'noProxy', '*!CHANGE_internalNetworkCIDRs*', null, '[config-yml]');
    }

  } else {
    // Delete proxy section
    $('code span.na').filter(function () {
      return (this.innerText === "proxy");
    }).each(function (index) {
      parent = $(this).parent();
      delete_elements($(this).prev(), 11);
      parent.html(parent.html().replaceAll(/\n\s+\n/g, "\n"));
    });
    updateTextInSnippet('[config-yml]', /\n\s*#[^#]+\n\s*proxy:\n/, "\n");
    updateTextInSnippet('[config-yml]', /\n\s+httpProxy: \<HTTP_PROXY_ADDRESS\>\n/, "\n");
    updateTextInSnippet('[config-yml]', /\n\s+httpsProxy: \<HTTPS_PROXY_ADDRESS\>\n/, "\n");
    updateTextInSnippet('[config-yml]', /\n\s+noProxy: \<NO_PROXY_LIST\>\n/, "\n");
  }

  if (!(noProxyAddressList && noProxyAddressList.length > 0)) {
    // Delete noProxy section
    $('code span.na').filter(function () {
      return (this.innerText === "noProxy");
    }).each(function (index) {
      parent = $(this).parent();
      delete_elements($(this), 2);
      parent.html(parent.html().replaceAll(/\n\s+\n/g, "\n"));
    });
    updateTextInSnippet('[config-yml]', /\n\s+noProxy: \<NO_PROXY_LIST\>\n/, "\n");
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
        updateTextInSnippet('[config-yml]', /(registryScheme: HTTP[S]?).+\<REGISTRY_CA\>\n---/s, "$1\n---");
      });

    }

    $('.highlight code').filter(function () {
      return this.innerText.match('<IMAGES_REPO_URI>');
    }).each(function () {
      $(this).text($(this).text().replace('<IMAGES_REPO_URI>', cleanedRegistryImagesRepo));
    });

    updateTextInSnippet('[docker-run-ce]', '<IMAGES_REPO_URI>', cleanedRegistryImagesRepo);
    updateTextInSnippet('[docker-run-windows-ce]', '<IMAGES_REPO_URI>', cleanedRegistryImagesRepo);
    updateTextInSnippet('[docker-login-windows]', '<IMAGES_REPO_URI>', cleanedRegistryImagesRepo);
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

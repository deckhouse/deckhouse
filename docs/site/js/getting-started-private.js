$(document).ready(function () {
  let enablePackagesProxy = sessionStorage.getItem('dhctl-packages-proxy-enabled')
  let proxyUsername = sessionStorage.getItem('dhctl-packages-proxy-username')
  let proxyPassword = sessionStorage.getItem('dhctl-packages-proxy-password')
  let proxyURI = sessionStorage.getItem('dhctl-packages-proxy-uri')
  let registryDockerCfg = sessionStorage.getItem('dhctl-registry-docker-cfg')
  let registryImagesRepo = sessionStorage.getItem('dhctl-registry-images-repo')
  let registrySchemeHTTP = sessionStorage.getItem('dhctl-registry-scheme-http')
  let registryCA = sessionStorage.getItem('dhctl-registry-ca')

  if ( enablePackagesProxy && enablePackagesProxy === "true" &&
       proxyUsername && proxyUsername.length > 0 &&
       proxyPassword && proxyPassword.length > 0 &&
       proxyURI && proxyURI.length > 0 ) {
    update_parameter('dhctl-packages-proxy-uri', 'uri', 'https://example.com', null, '[config-yml]');
    update_parameter('dhctl-packages-proxy-username', 'username', '<PROXY-USERNAME>', null, '[config-yml]');
    update_parameter('dhctl-packages-proxy-password', 'password', '<PROXY-PASSWORD>', null, '[config-yml]');
  } else if (enablePackagesProxy && enablePackagesProxy === "true" && proxyURI && proxyURI.length > 0) {
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
      updateTextInSnippet('[config-yml]', /packagesProxy.+<PROXY-PASSWORD>\n---/s, "---");
    });
  }

  if (registryImagesRepo && registryImagesRepo.length > 0) {
    update_parameter('dhctl-registry-docker-cfg', 'registryDockerCfg', '<YOUR_PRIVATE_ACCESS_STRING_IS_HERE>', null, '[config-yml]');
    update_parameter('dhctl-registry-images-repo', 'imagesRepo', '<IMAGES_REPO_URI>', null, '[config-yml]');
    update_parameter('dhctl-registry-ca', 'registryCA', '<REGISTRY_CA>', null, '[config-yml]', 4);
    if (registrySchemeHTTP && registrySchemeHTTP === 'true') {
      update_parameter('http', 'registryScheme', 'https', null, null);
      updateTextInSnippet('[config-yml]', /registryScheme: https.+\n---/s, "registryScheme: http\n---");
    }

    if ((registrySchemeHTTP && registrySchemeHTTP === 'true') || !registryCA || (registryCA && registryCA.length < 1)) {
      // delete the registryCA parameter
      $('code span.na').filter(function () {
        return (this.innerText === "registryScheme");
      }).each(function (index) {
        delete_elements($(this).next().next().next(), 3);
        updateTextInSnippet('[config-yml]', /(registryScheme: http[s]?).+\n---/s, "$1\n---");
      });

    }
    $('.highlight code').filter(function () {
      return this.innerText.match('<IMAGES_REPO_URI>');
    }).each(function () {
      $(this).text($(this).text().replace('<IMAGES_REPO_URI>', registryImagesRepo));
    });

    updateTextInSnippet('[docker-login-ce]', '<IMAGES_REPO_URI>', registryImagesRepo);
  }

  // delete empty lines in snippet
  $('div.snippetcut code').each(function () {
    let text = this.innerHTML;
    this.innerHTML = text.replace(/\s+\n(<span class="nn">---)/gs,"\n$1");
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

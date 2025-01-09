(function () {
  // Get array of strings to highlight in a snippet.
  // Don't highlight if any quotations.
  // Don't highlight short words (less than 3 chars).
  function getHighlights(input) {
    if (input.search(/['"]/) < 0 ) {
      return input.split(' ').filter(function (item) {
        return item.length > 2;
      }).sort(function (a, b) {
        return b.length - a.length
      });
    } else {
      return [input]
    }
  }

  function getNormalizedContent(content) {
    let resultMaxLength = 250;

    if (content.length <= resultMaxLength) return content;

    let result = content.substring(0, resultMaxLength);
    let lastSpaceIndex = result.lastIndexOf(' ');
    if (resultMaxLength - lastSpaceIndex < 30) {
      result = result.substring(0, lastSpaceIndex + 1);
    }

    result = result.replace(/[^A-Za-zА-Яа-я0-9_]+$/, '') + '…';

    return result;
  }

  function getSnippet(content, highlights = []) {
    let resultMaxLength = 250;
    let result = '';
    let tmp = '';

    if ( highlights.length ) {
      let highlightPosition = 0;
      for (let i = 0; i < highlights.length; i++) {
        const re = new RegExp(`(${highlights[i].replace(/[.*+?^${}()|[\]\\]/g, '')})`, 'gi');
        let pos = content.search(re);
        if ( pos > 0 ) {
          highlightPosition = pos; break;
        }
      }

      let blockStartPosition = 0;
      let blockEndPosition = content.length;
      if ( highlightPosition + resultMaxLength / 2 <= content.length ) {
        if ( highlightPosition - resultMaxLength / 2 >= 0 ) {
          blockEndPosition = highlightPosition + resultMaxLength / 2;
        } else {
          blockEndPosition = highlightPosition + resultMaxLength / 2 - (highlightPosition - resultMaxLength / 2);
          if ( blockEndPosition > content.length ) blockEndPosition = content.length;
        }
      }
      if ( blockEndPosition - resultMaxLength > 0 ) {
        blockStartPosition = blockEndPosition - resultMaxLength;
        if ( blockStartPosition < 10 ) blockStartPosition = 0;
      }

        result = content.substring(blockStartPosition, blockEndPosition);

      if ( blockStartPosition > 0 ) {
        let firstSpaceIndex = result.indexOf(' ');
        if ( firstSpaceIndex < 30 ) {
          result = result.substring(firstSpaceIndex);
        }
        result = '…' + result;
      }

      if ( blockEndPosition < content.length ) {
        let lastSpaceIndex = result.lastIndexOf(' ');
        if ( resultMaxLength - lastSpaceIndex < 30 ) {
          result = result.substring(0, lastSpaceIndex + 1);
        }
        result += '…';
      }

      // Add class to highlighted elements
      highlights.forEach((item) => {
        const re = new RegExp(`(${item.replace(/[.*+?^${}()|[\]\\]/g, '')})`, 'gi');
        result = result.replace(re, `<span class="highlight">$1</span>`);
      })

    } else {
      if (content.length <= resultMaxLength) return content;

      result = content.substring(0, resultMaxLength);

      let lastSpaceIndex = result.lastIndexOf(' ');
      if (resultMaxLength - lastSpaceIndex < 30) {
        result = result.substring(0, lastSpaceIndex + 1);
      }

      result = result.replace(/[^A-Za-zА-Яа-я0-9_]+$/, '') + '…';
    }

    return result;

  }

  function displayDocumentsSearchResults(results, store, containerClass, highlights = []) {

    if (results && results.length > 0) { // Are there any results?
      $(containerClass + ' .searchV2__results-counter-data').text(results.length);
      $(containerClass + ' .searchV2__results-counter').addClass('active');
      $(containerClass + ' .searchV2__results-absent-block').removeClass('active');
      let appendString = '';

      for (let i = 0; i < results.length; i++) {  // Iterate over the results
        let item = store.find(function (element) {
          return element.url === results[i].ref;
        });
        appendString += `
           <li>
             <div class="searchV2__item-title-block">
               <a href="./${item.url.replace(/^\//, '')}">
                 <span class="searchV2__item-title-header">${item.title}</span>
               </a>
             </div>
             <div class="searchV2__item-content-block">
                ${getSnippet(item.content, highlights)}
             </div>
           </li>`;
      }
      $(containerClass + ' .searchV2__results').html(appendString);
    } else {
      console.log("There are no results.");
      $(containerClass + ' .searchV2__results-counter').removeClass('active');
      $(containerClass + ' .searchV2__results-absent-block').addClass('active');
    }
  }

  function displayParametersSearchResults(rawResults, store, containerClass, highlights = []) {
    if (rawResults && rawResults.length > 0) {
      $(containerClass + ' .searchV2__results-counter-data').text(rawResults.length);
      $(containerClass + ' .searchV2__results-counter').addClass('active');
      $(containerClass + ' .searchV2__results-absent-block').removeClass('active');
      let appendString = '';

      results = rawResults.filter(function(resultItem){
        return store.find(function (element) {
          return element.url === resultItem.ref;
        }).isResource === 'true';
      }).concat(
      rawResults.filter(function(resultItem){
        return ! store.find(function (element) {
          return element.url === resultItem.ref;
        }).isResource;
      })
      );

      for (let i = 0; i < results.length; i++) {
        let deprecatedString = '';
        let title = '';
        let item = store.find(function (element) {
          return element.url === results[i].ref;
        });

        if (item.deprecated === 'true') {
          deprecatedString = `<span class="deprecated">Deprecated</span>`
        }

        if ( item.isResource === 'true' ) {
          // Add text prefix for resource / custom resource.
          title = item.name
        } else if ( item.resName && item.resName.length > 0 )
          title = `${item.resName}: ${item.name}`
        else
          title = item.name

        let titleClass='searchV2__item-title-header';
        if (item.isResource === 'true') titleClass += ' searchV2__item-custom-resource';

        appendString += `
           <li>
             <div class="searchV2__item-title-block">
               <a href="./${item.url.replace(/^\//, '')}">
                 <span class="${titleClass}">${title}</span>
               </a>
               ${deprecatedString}
             </div>
             <div class="searchV2__item-content-block">
               <span class="searchV2__item-parameter-path">${item.path}</span>
               <p>${getSnippet(item.content, highlights)}</p>
             </div>
           </li>`;
      }
      $(containerClass + ' .searchV2__results').html(appendString);
    } else {
      console.log("There are no results.");
      $(containerClass + ' .searchV2__results-counter').removeClass('active');
      $(containerClass + ' .searchV2__results-absent-block').addClass('active');
    }
  }

  function getQueryVariable(variable) {
    let query = window.location.search.substring(1);
    let vars = query.split('&');

    for (let i = 0; i < vars.length; i++) {
      let pair = vars[i].split('=');

      if (pair[0] === variable) {
        return decodeURIComponent(pair[1].replace(/\+/g, '%20'));
      }
    }
  }

  // Get the search string and escape ':' as it defines a field to use and may lead to an error, e.g., when searching for 'kind: user'
  const searchTerm = getQueryVariable('query').replace(/([^\\]):/g, '$1\\:');
  //  Get a string to highlight in snippets
  const stringsToHighlight = getHighlights(searchTerm.replace(/\\:/g, ':'));

  if (searchTerm && searchTerm.length > 0) {
    document.getElementById('search-box').setAttribute("value", searchTerm);

    // Initalize lunr with the fields it will be searching on. I've given title
    // a boost of 10 to indicate matches on this field are more important.
    let documentsIdx = lunr(function () {
      this.use(lunr.multiLanguage('en', 'ru'))
      this.ref('url')
      this.field('title', {boost: 10})
      this.field('keywords', {boost: 20})
      this.field('content')

      documents.forEach(function (doc) {
        this.add(doc)
      }, this)
    });

    let parametersIdx = lunr(function () {
      this.use(lunr.multiLanguage('en', 'ru'))
      this.ref('url')
      this.field('name', {boost: 10})
      this.field('keywords', {boost: 20})
      this.field('content')

      parameters.forEach(function (doc) {
        this.add(doc)
      }, this)
    });

    let resultsDocuments = documentsIdx.search(searchTerm); // Get lunr to perform a search on documents
    let resultsParameters = parametersIdx.search(searchTerm); // Get lunr to perform a search on parameters
    displayDocumentsSearchResults(resultsDocuments, documents, '.searchV2__documents', stringsToHighlight);
    displayParametersSearchResults(resultsParameters, parameters, '.searchV2__parameters', stringsToHighlight);
    if ( $('.searchV2__documents .searchV2__results-absent-block').hasClass('active') && $('.searchV2__parameters .searchV2__results-absent-block').hasClass('active') ) {
        $('.searchV2').css({ 'flex-direction': 'column', 'gap': '0' });
    };

  } else {
    $('.searchV2 .searchV2__results-absent-block').addClass('active');
    $('.searchV2').css({ 'flex-direction': 'column', 'gap': '0' });
  }
})();

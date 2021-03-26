(function() {
  function displaySearchResults(results, store) {
    var searchResults = document.getElementById('search-results');

    if (results.length) { // Are there any results?
      document.getElementById('search__results__counts__data').innerText = results.length;
      $('.search__results__counts').addClass('active');
      $('.search__results__absent').removeClass('active');
      var appendString = '';

      for (var i = 0; i < results.length; i++) {  // Iterate over the results
        // var item = store[results[i].ref];
        var item = store.find(function(element) {
              return element.url === results[i].ref;
            });

        appendString += '<li><a href="' + item.url + '"><h3>' + item.title + '</h3></a>';
        appendString += '<p>' + item.content.substring(0, 150) + '...</p></li>';
      }

      searchResults.innerHTML = appendString;
    } else {
      $('.search__results__counts').removeClass('active');
      $('.search__results__absent').addClass('active');
    }
  }

  function getQueryVariable(variable) {
    var query = window.location.search.substring(1);
    var vars = query.split('&');

    for (var i = 0; i < vars.length; i++) {
      var pair = vars[i].split('=');

      if (pair[0] === variable) {
        return decodeURIComponent(pair[1].replace(/\+/g, '%20'));
      }
    }
  }

  var searchTerm = getQueryVariable('query');

  if (searchTerm) {
    document.getElementById('search-box').setAttribute("value", searchTerm);

    // Initalize lunr with the fields it will be searching on. I've given title
    // a boost of 10 to indicate matches on this field are more important.
    var idx = lunr(function () {
      this.use(lunr.multiLanguage('en', 'ru'))
      this.ref('url')
      this.field('title', { boost: 10 })
      this.field('content')

    documents.forEach(function (doc) {
        this.add(doc)
      }, this)
    });

    var results = idx.search(searchTerm); // Get lunr to perform a search
      displaySearchResults(results, documents); // We'll write this in the next section
  } else {
  $('.search__results__absent').addClass('active');
  }
})();

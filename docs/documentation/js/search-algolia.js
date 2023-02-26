const search = instantsearch({
  indexName: 'deckhouse',
  searchClient: algoliasearch('IQGZ6EU4TG', 'fc448cf6ce373f5963ef53f2c942151e'),
});

search.addWidgets([
  instantsearch.widgets.searchBox({
    container: '#searchbox',
    searchAsYouType: true,
    autofocus: true,
  }),

  instantsearch.widgets.hits({
    container: '#hits',
    templates: {
      item: `
        <div>
          <!-- <img src="{{image}}" align="left" alt="{{name}}" /> -->
          <a href="{{url}}" align="left" alt="{{title}}">{{title}}</a>
          <div class="hit-description">
            {{#helpers.snippet}}{ "attribute": "content" }{{/helpers.snippet}}
          </div>
        </div>
      `,
    },
  }),
  instantsearch.widgets.pagination({
    container: '#pagination',
  }),
]);

search.start();

class ModuleSearch {
  constructor(options = {}) {
    this.searchInput = document.getElementById('search-input');
    this.searchResults = document.getElementById('search-results');
    this.searchIndex = null;
    this.searchData = null;
    this.lunrIndex = null;
    this.fuseIndex = null;
    this.searchDictionary = [];
    this.lastQuery = '';
    this.currentResults = {
      config: [],
      other: []
    };
    this.displayedCounts = {
      config: 5,
      other: 5
    };
    this.isDataLoaded = false;

    // Configuration options
    this.options = {
      searchIndexPath: '/modules/search-embedded-modules-index.json',
      ...options
    };

    // Initialize i18n
    this.initI18n();

    this.init();
  }

    initI18n() {
    // Get current page language from HTML lang attribute
    this.currentLang = document.documentElement.lang || 'en';

    // i18n dictionary for Russian and English
    this.i18n = {
      en: {
        api: 'API',
        documentation: 'Documentation',
        showMore: 'Show more',
        loading: 'Loading search index...',
        ready: 'What are we looking for?',
        noResults: `Results for "{query}" not found.\nTry different keywords or check your spelling.`,
        error: 'An error occurred during search.',
        showMorePattern: 'Show {count} more'
      },
      ru: {
        api: 'API',
        documentation: 'Документация',
        showMore: 'Показать еще',
        loading: 'Загрузка поискового индекса...',
        ready: 'Что ищем?',
        noResults: "Нет результатов для \"{query}\".\nПопробуйте другие ключевые слова или проверьте правописание.",
        error: 'An error occurred during search.',
        showMorePattern: 'Показать еще {count}'
      }
    };

    // Default to English if language not supported
    if (!this.i18n[this.currentLang]) {
      this.currentLang = 'en';
    }
  }

    // Get translated text
  t(key, params = {}) {
    let text = this.i18n[this.currentLang][key] || this.i18n.en[key] || key;

    // Replace parameters in the text
    Object.keys(params).forEach(param => {
      text = text.replace(`{${param}}`, params[param]);
    });

    return text;
  }

  // Refresh language detection
  refreshLanguageDetection() {
    // Re-check language from HTML lang attribute only
    const htmlLang = document.documentElement.lang;

    if (htmlLang === 'ru') {
      this.currentLang = 'ru';
    } else if (htmlLang === 'en') {
      this.currentLang = 'en';
    }
  }

  async init() {
    this.setupEventListeners();

    // Hide search results by default
    this.searchResults.style.display = 'none';
  }

    setupEventListeners() {
    // Load search index on focus
    this.searchInput.addEventListener('focus', () => {
      // Show loading state when user first focuses on search
      if (!this.isDataLoaded) {
        this.showLoading();
        this.searchInput.disabled = true;
        this.searchInput.placeholder = this.t('loading');
      }
      this.loadSearchIndex();
      // Show search results container when focused (even if empty)
      this.searchResults.style.display = 'flex';
    });

    // Hide results when input loses focus (unless clicking on results)
    this.searchInput.addEventListener('blur', (e) => {
      // Use setTimeout to allow click events on results to fire first
      setTimeout(() => {
        // Check if the user clicked on search results or if focus is still within search area
        const activeElement = document.activeElement;
        const isClickingOnSearch = this.searchResults.contains(activeElement) ||
                                  this.searchInput.contains(activeElement) ||
                                  activeElement.closest('.searchV3');

        // Also check if the blur was caused by clicking on search elements
        const relatedTarget = e.relatedTarget;
        const isBlurToSearch = relatedTarget && (
          this.searchResults.contains(relatedTarget) ||
          this.searchInput.contains(relatedTarget) ||
          relatedTarget.closest('.searchV3')
        );

        // Don't hide search results if index is still loading or if there are loading/error messages
        const hasLoadingOrError = this.searchResults.querySelector('.loading, .no-results');
        if (!isClickingOnSearch && !isBlurToSearch && !hasLoadingOrError) {
          this.searchResults.style.display = 'none';
        }
      }, 150);
    });

    this.searchInput.addEventListener('input', (e) => {
      // Don't allow searching until index is loaded
      if (!this.isDataLoaded) {
        return;
      }
      
      const query = e.target.value.trim();
      if (query.length > 0) {
        // Show search results when user starts typing
        this.searchResults.style.display = 'flex';
        this.handleSearch(query);
      } else {
        // Hide search results when search is cleared, but not if there are loading/error messages
        const hasLoadingOrError = this.searchResults.querySelector('.loading, .no-results');
        if (!hasLoadingOrError) {
          this.searchResults.style.display = 'none';
        }
      }
    });

    this.searchInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        // Don't allow searching until index is loaded
        if (!this.isDataLoaded) {
          return;
        }
        
        const query = e.target.value.trim();
        if (query.length > 0) {
          this.searchResults.style.display = 'flex';
          this.handleSearch(query);
        }
      }

      // Close search results with Escape key
      if (e.key === 'Escape') {
        this.searchResults.style.display = 'none';
        this.searchInput.blur();
      }
    });

    // Close search results when clicking outside
    document.addEventListener('click', (e) => {
      // Check if the click is on the search input or search results
      const isClickOnSearch = this.searchInput.contains(e.target) ||
                             this.searchResults.contains(e.target) ||
                             e.target.closest('.searchV3');

      // Don't close if clicking on search elements
      if (isClickOnSearch) {
        return;
      }

      // Close search results when clicking outside, but not if there are loading/error messages
      const hasLoadingOrError = this.searchResults.querySelector('.loading, .no-results');
      if (!hasLoadingOrError) {
        this.searchResults.style.display = 'none';
      }
    });

    // Prevent search results from closing when clicking on buttons inside results
    this.searchResults.addEventListener('click', (e) => {
      // If clicking on a button or interactive element, prevent closing
      if (e.target.tagName === 'BUTTON' ||
          e.target.closest('button') ||
          e.target.closest('.tile__pagination') ||
          e.target.closest('.more-button')) {
        e.stopPropagation();
        e.preventDefault();

        // Keep focus on search input to prevent blur from hiding results
        this.searchInput.focus();
      }
    });
  }

  async loadSearchIndex() {
    if (this.isDataLoaded) {
      return; // Already loaded
    }

    try {
      this.showLoading();

      // Check if searchIndexPath contains multiple comma-separated paths
      const indexPaths = this.options.searchIndexPath.split(',').map(path => path.trim());

      if (indexPaths.length === 1) {
        // Single index file
        const response = await fetch(indexPaths[0]);
        if (!response.ok) {
          throw new Error(`Failed to load search index: ${response.status}`);
        }
        this.searchData = await response.json();
      } else {
        // Multiple index files - load and merge them
        console.log(`Loading ${indexPaths.length} search index files:`, indexPaths);

        const responses = await Promise.all(
          indexPaths.map(async (path) => {
            try {
              const response = await fetch(path);
              if (!response.ok) {
                console.warn(`Failed to load search index: ${path} (${response.status})`);
                return { documents: [], parameters: [] };
              }
              return await response.json();
            } catch (error) {
              console.warn(`Error loading search index: ${path}`, error);
              return { documents: [], parameters: [] };
            }
          })
        );

        // Merge all search indexes
        this.searchData = {
          documents: [],
          parameters: []
        };

        responses.forEach((indexData, index) => {
          if (indexData && indexData.documents) {
            this.searchData.documents = this.searchData.documents.concat(indexData.documents);
          }
          if (indexData && indexData.parameters) {
            this.searchData.parameters = this.searchData.parameters.concat(indexData.parameters);
          }
        });

        // console.log(`Merged search data: ${this.searchData.documents.length} documents, ${this.searchData.parameters.length} parameters`);
      }

      // Refresh language detection before building index
      this.refreshLanguageDetection();

      this.buildLunrIndex();
      this.buildSearchDictionary();
      this.buildFuseIndex();
      this.isDataLoaded = true;
      this.hideLoading();

      // Re-enable search input
      this.searchInput.disabled = false;
      this.searchInput.placeholder = this.t('ready');

      // Keep focus on search input after loading
      this.searchInput.focus();

      // Show message that search index is loaded and ready
      this.showMessage(this.t('ready'));
    } catch (error) {
      console.error('Error loading search index:', error);
      // Re-enable search input even on error
      this.searchInput.disabled = false;
      this.searchInput.placeholder = this.t('ready');
      
      // Keep focus on search input after error
      this.searchInput.focus();
      
      this.showError('Failed to load search index. Please try again later.');
    }
  }

  buildLunrIndex() {
    const searchData = this.searchData;

    // Use multilingual support for Russian, default for English
    if (this.currentLang === 'ru' && typeof lunr.multiLanguage !== 'undefined') {
      // Use Russian language support with lunr.multiLanguage
      this.lunrIndex = lunr(function() {
        this.use(lunr.multiLanguage('en', 'ru'));
        this.field('title', { boost: 10 });
        this.field('keywords', { boost: 8 });
        this.field('module', { boost: 6 });
        this.field('summary', { boost: 3 });
        this.field('content', { boost: 1 });
        this.ref('id');

        // Add documents from the documents array
        if (searchData.documents) {
          searchData.documents.forEach((doc, index) => {
            this.add({
              id: `doc_${index}`,
              title: doc.title || '',
              keywords: doc.keywords || '',
              module: doc.module || '',
              summary: doc.summary || '',
              content: doc.content || '',
              url: doc.url || '',
              moduletype: doc.moduletype || '',
              type: 'document'
            });
          });
        }

        // Add parameters from the parameters array
        if (searchData.parameters) {
          searchData.parameters.forEach((param, index) => {
            this.add({
              id: `param_${index}`,
              title: param.name || '',
              keywords: param.keywords || '',
              module: param.module || '',
              resName: param.resName || '',
              content: param.content || '',
              url: param.url || '',
              moduletype: param.moduletype || '',
              type: 'parameter'
            });
          });
        }
      });

      // console.log('Built search index with Russian multilingual support');
    } else {
      // Use default English language support
      this.lunrIndex = lunr(function() {
        this.field('title', { boost: 10 });
        this.field('keywords', { boost: 8 });
        this.field('module', { boost: 6 });
        this.field('summary', { boost: 3 });
        this.field('content', { boost: 1 });
        this.ref('id');

        // Add documents from the documents array
        if (searchData.documents) {
          searchData.documents.forEach((doc, index) => {
            this.add({
              id: `doc_${index}`,
              title: doc.title || '',
              keywords: doc.keywords || '',
              module: doc.module || '',
              summary: doc.summary || '',
              content: doc.content || '',
              url: doc.url || '',
              type: 'document'
            });
          });
        }

        // Add parameters from the parameters array
        if (searchData.parameters) {
          searchData.parameters.forEach((param, index) => {
            this.add({
              id: `param_${index}`,
              title: param.name || '',
              keywords: param.keywords || '',
              module: param.module || '',
              resName: param.resName || '',
              content: param.content || '',
              url: param.url || '',
              type: 'parameter'
            });
          });
        }
      });

      // console.log('Built search index with default English support');
    }
  }

  buildSearchDictionary() {
    const dictionary = new Set();

    // Extract searchable terms from documents
    if (this.searchData.documents) {
      this.searchData.documents.forEach(doc => {
        // Add title words
        if (doc.title) {
          this.extractWords(doc.title).forEach(word => dictionary.add(word));
        }
        // Add keywords
        if (doc.keywords && Array.isArray(doc.keywords)) {
          doc.keywords.forEach(keyword => {
            this.extractWords(keyword).forEach(word => dictionary.add(word));
          });
        }
        // Add module name
        if (doc.module) {
          this.extractWords(doc.module).forEach(word => dictionary.add(word));
        }
        // Add summary words
        if (doc.summary) {
          this.extractWords(doc.summary).forEach(word => dictionary.add(word));
        }
      });
    }

    // Extract searchable terms from parameters
    if (this.searchData.parameters) {
      this.searchData.parameters.forEach(param => {
        // Add parameter name
        if (param.name) {
          this.extractWords(param.name).forEach(word => dictionary.add(word));
        }
        // Add keywords
        if (param.keywords && Array.isArray(param.keywords)) {
          param.keywords.forEach(keyword => {
            this.extractWords(keyword).forEach(word => dictionary.add(word));
          });
        }
        // Add module name
        if (param.module) {
          this.extractWords(param.module).forEach(word => dictionary.add(word));
        }
        // Add resource name
        if (param.resName) {
          this.extractWords(param.resName).forEach(word => dictionary.add(word));
        }
      });
    }

    // Convert to array and sort alphabetically
    this.searchDictionary = Array.from(dictionary)
      .filter(word => word.length >= 2) // Filter out very short words (reduced from 3 to 2)
      .sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));

    console.log(`Built search dictionary with ${this.searchDictionary.length} unique terms`);
  }

  extractWords(text) {
    if (!text) return [];

    // Extract words from text, handling various separators and special characters
    // Use a better regex that properly handles Cyrillic characters
    const words = text
      .toLowerCase()
      .replace(/[^\p{L}\p{N}\s-]/gu, ' ') // Unicode-aware: keep letters, numbers, spaces, hyphens
      .replace(/[-_]/g, ' ') // Replace hyphens and underscores with spaces
      .split(/\s+/)
      .filter(word => word.length >= 2) // Filter out very short words
      .filter(word => !/^\d+$/.test(word)) // Filter out pure numbers
      .filter(word => /[\p{L}]/u.test(word)); // Only keep words that contain letters

    // Russian words extraction working properly with Unicode regex

    return words;
  }

  buildFuseIndex() {
    if (typeof Fuse === 'undefined') {
      console.warn('Fuse.js not available, fuzzy search disabled');
      return;
    }

    // Building Fuse.js index for fuzzy search

    // Create Fuse.js index for fuzzy search
    this.fuseIndex = new Fuse(this.searchDictionary, {
      threshold: 0.8, // Higher threshold = more lenient matching (0.0 = exact, 1.0 = match anything)
      distance: 100,  // Maximum distance for fuzzy matching
      includeScore: true,
      minMatchCharLength: 2,
      // Better support for Cyrillic characters
      ignoreLocation: true,
      findAllMatches: true,
      useExtendedSearch: false
    });

    console.log('Built Fuse.js index for fuzzy search');
  }

  getFuzzySuggestions(query) {
    if (!this.fuseIndex || !query.trim()) {
      return [];
    }

    // Get fuzzy matches from the dictionary
    let fuzzyResults = this.fuseIndex.search(query);

    // Check if query contains Russian characters and use fallback if needed
    const hasRussian = /[а-яё]/i.test(query);
    if (hasRussian && fuzzyResults.length === 0) {
      // Fallback for Russian: use simple character-based similarity
      fuzzyResults = this.getRussianFuzzySuggestions(query);
    }

    // Return top 5 suggestions with scores
    return fuzzyResults.slice(0, 5);
  }

  getRussianFuzzySuggestions(query) {
    // Fallback method for Russian text when Fuse.js doesn't work well
    const queryLower = query.toLowerCase();
    const results = [];

    // Get Russian terms from dictionary
    const russianTerms = this.searchDictionary.filter(term => /[а-яё]/i.test(term));

    for (const term of russianTerms) {
      const termLower = term.toLowerCase();

      // Calculate simple similarity score
      let score = 0;

      // Check for exact match first
      if (termLower === queryLower) {
        score = 1.0;
      }
      // Check for substring matches
      else if (termLower.includes(queryLower)) {
        score = 0.8;
      } else if (queryLower.includes(termLower)) {
        score = 0.7;
      } else {
        // Calculate character-based similarity
        const similarity = this.calculateRussianSimilarity(queryLower, termLower);
        if (similarity > 0.2) { // Lowered threshold
          score = similarity;
        }
      }

      if (score > 0.2) { // Lowered threshold
        results.push({
          item: term,
          score: score
        });
      }
    }

    // Sort by score and return
    return results.sort((a, b) => b.score - a.score);
  }

  calculateRussianSimilarity(str1, str2) {
    // Simple Levenshtein distance for Russian text
    const matrix = [];
    const len1 = str1.length;
    const len2 = str2.length;

    for (let i = 0; i <= len2; i++) {
      matrix[i] = [i];
    }

    for (let j = 0; j <= len1; j++) {
      matrix[0][j] = j;
    }

    for (let i = 1; i <= len2; i++) {
      for (let j = 1; j <= len1; j++) {
        if (str2.charAt(i - 1) === str1.charAt(j - 1)) {
          matrix[i][j] = matrix[i - 1][j - 1];
        } else {
          matrix[i][j] = Math.min(
            matrix[i - 1][j - 1] + 1, // substitution
            matrix[i][j - 1] + 1,     // insertion
            matrix[i - 1][j] + 1      // deletion
          );
        }
      }
    }

    const distance = matrix[len2][len1];
    const maxLength = Math.max(len1, len2);
    return 1 - (distance / maxLength);
  }

  clearFuzzySearchMessages() {
    // Remove any existing fuzzy search messages and suggestions
    const existingMessages = this.searchResults.querySelectorAll('.fuzzy-search-message, .fuzzy-suggestions');
    existingMessages.forEach(message => message.remove());
  }

  async handleSearch(query) {
    if (!query.trim()) {

      this.lastQuery = '';
      this.resetPagination();
      return;
    }

    // Load search data on demand if not already loaded
    if (!this.isDataLoaded) {
      await this.loadSearchIndex();
    }

    if (!this.lunrIndex) {
      this.showError('Search index not loaded yet.');
      return;
    }

    try {
      this.lastQuery = query;
      this.resetPagination();

      // Clear any existing fuzzy search messages
      this.clearFuzzySearchMessages();

      // First try exact search
      let results = this.lunrIndex.search(query);
      let highlightQuery = query; // Default to original query for highlighting

      // If no results and fuzzy search is available, try fuzzy search
      if (results.length === 0 && this.fuseIndex) {
        const fuzzySuggestions = this.getFuzzySuggestions(query);

          if (fuzzySuggestions.length > 0) {
            // Try searching with the best fuzzy suggestion
            const bestSuggestion = fuzzySuggestions[0].item;
            // Using fuzzy suggestion for better results
            results = this.lunrIndex.search(bestSuggestion);
            // Use the fuzzy suggestion for highlighting
            highlightQuery = bestSuggestion;
          }
      }

      // If still no results, try searching with individual words from fuzzy suggestions
      if (results.length === 0 && this.fuseIndex) {
        const fuzzySuggestions = this.getFuzzySuggestions(query);
        for (const suggestion of fuzzySuggestions.slice(0, 3)) { // Try top 3 suggestions
          const wordResults = this.lunrIndex.search(suggestion.item);
          if (wordResults.length > 0) {
            results = wordResults;
            // Use the fuzzy suggestion for highlighting
            highlightQuery = suggestion.item;
            break;
          }
        }
      }

      // Apply additional boosting for parameters and module name matches
      const boostedResults = results.map(result => {
        const docId = result.ref;
        let doc;

        // Determine which array the result comes from
        if (docId.startsWith('doc_')) {
          const index = parseInt(docId.replace('doc_', ''));
          doc = this.searchData.documents[index];
        } else if (docId.startsWith('param_')) {
          const index = parseInt(docId.replace('param_', ''));
          doc = this.searchData.parameters[index];
        }

        if (!doc) return result;

        let boost = 1;

        // Check if the search query matches the module name
        const queryLower = query.toLowerCase();
        const moduleLower = (doc.module || '').toLowerCase();

        if (moduleLower && moduleLower.includes(queryLower)) {
          boost *= 1.8; // Strong boost for module name matches
        }

        // Apply existing parameter boosting logic
        if (doc.type === 'parameter' && doc.content && doc.content.includes('resources__prop_name')) {
          boost *= 1.5; // Additional boost for parameters with properties
        } else if (doc.type === 'parameter') {
          boost *= 1.2; // Moderate boost for parameters
        }

        return {
          ...result,
          score: result.score * boost
        };
      });

      // Sort by boosted score
      boostedResults.sort((a, b) => b.score - a.score);

      // Store current results and display them
      this.currentResults = this.groupResults(boostedResults);
      this.currentHighlightQuery = highlightQuery; // Store the query to use for highlighting
      this.displayResults();

    } catch (error) {
      console.error('Search error:', error);
      this.showError('An error occurred during search.');
    }
  }

  groupResults(results) {
    const configResults = [];
    const otherResults = [];

    results.forEach(result => {
      const docId = result.ref;
      let doc;

      // Determine which array the result comes from
      if (docId.startsWith('doc_')) {
        const index = parseInt(docId.replace('doc_', ''));
        doc = this.searchData.documents[index];
        doc.type = 'document';
      } else if (docId.startsWith('param_')) {
        const index = parseInt(docId.replace('param_', ''));
        doc = this.searchData.parameters[index];
        doc.type = 'parameter';
      }

      if (doc) {
        // Configuration results come from parameters array
        if (doc.type === 'parameter') {
          configResults.push(result);
        } else {
          // Other documentation comes from documents array
          otherResults.push(result);
        }
      }
    });

    return {
      config: configResults,
      other: otherResults
    };
  }

  displayResults() {
    if (this.currentResults.config.length === 0 && this.currentResults.other.length === 0) {
      this.showNoResults(this.lastQuery);
      return;
    }

    let resultsHtml = '';

    // Display configuration results first
    if (this.currentResults.config.length > 0) {
      resultsHtml += `
        <div class="results-group">
          <div class="results-group-header">${this.t('api')}</div>
          ${this.renderResultGroup(this.currentResults.config, this.currentHighlightQuery || this.lastQuery, 'config')}
        </div>
      `;
    }

    // Display other results
    if (this.currentResults.other.length > 0) {
      resultsHtml += `
        <div class="results-group">
          <div class="results-group-header">${this.t('documentation')}</div>
          ${this.renderResultGroup(this.currentResults.other, this.currentHighlightQuery || this.lastQuery, 'other')}
        </div>
      `;
    }

    this.searchResults.innerHTML = resultsHtml;
  }

  renderResultGroup(results, query, groupType) {
    const displayedCount = this.displayedCounts[groupType];
    const topResults = results.slice(0, displayedCount);

    let html = '';

    // Render visible results
    topResults.forEach(result => {
      const docId = result.ref;
      let doc;

      // Determine which array the result comes from
      if (docId.startsWith('doc_')) {
        const index = parseInt(docId.replace('doc_', ''));
        doc = this.searchData.documents[index];
      } else if (docId.startsWith('param_')) {
        const index = parseInt(docId.replace('param_', ''));
        doc = this.searchData.parameters[index];
      }

      if (!doc) return;

      let title, summary, module, description;

      if (groupType === 'config') {
        // For configuration results (parameters)
        title = this.highlightText(doc.name || '', query);
        // summary = this.highlightText(doc.resName || '', query);
        module = doc.module ? `<div class="result-module">${doc.module}</div>` : '';
        if (doc.resName != doc.name) {
          module += doc.resName ? `<div class="result-module">${doc.resName}</div>` : '';
        }
        // description = this.highlightText(doc.content || '', query);
        description = this.highlightText(this.getRelevantContentSnippet(doc.content || '', query) || '', query);
      } else {
        // For other documentation
        title = this.highlightText(doc.title || '', query);
        // summary = this.highlightText(doc.summary || '', query);
        module = doc.module ? `<div class="result-module">${doc.module}</div>` : '';
        // description = summary || this.getRelevantContentSnippet(doc.content || '', query);
        description = this.highlightText(this.getRelevantContentSnippet(doc.content || '', query) || '', query);
      }

      html += `
        <a href="${this.buildTargetUrl(doc.url, doc.moduletype)}" class="result-item">
          <div class="result-title">${title}</div>
          ${module}
          <div class="result-description">${description}</div>
        </a>
      `;
    });

    // Add "More" button if there are more results to show
    if (displayedCount < results.length) {
      html += `
        <button class="tile__pagination" onclick="window.moduleSearch.loadMore('${groupType}')">
          <p class="tile__pagination--descr">${this.t('showMorePattern', { count: Math.min(5, results.length - displayedCount) })}</p>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path fill-rule="evenodd" clip-rule="evenodd" d="M8 1C8.55229 1 9 1.44772 9 2V7L14 7C14.5523 7 15 7.44772 15 8C15 8.55229 14.5523 9 14 9L9 9L9 14C9 14.5523 8.55229 15 8 15C7.44772 15 7 14.5523 7 14L7 9H2C1.44772 9 1 8.55229 1 8C1 7.44772 1.44772 7 2 7L7 7L7 2C7 1.44772 7.44772 1 8 1Z" fill="#0D69F2"/>
          </svg>
        </button>
      `;
    }

    return html;
  }

  loadMore(groupType) {
    if (groupType === 'config' || groupType === 'other') {
      this.displayedCounts[groupType] += 5;
      this.displayResults();
    }
  }

  resetPagination() {
    this.displayedCounts = {
      config: 5,
      other: 5
    };
  }

  getRelevantContentSnippet(content, query) {
    if (!content || !query) return '';

    // Helper function to truncate text without cutting words
    const truncateText = (text, maxLength) => {
      if (text.length <= maxLength) return text;

      // Find the last space before maxLength
      let truncated = text.substring(0, maxLength);
      const lastSpaceIndex = truncated.lastIndexOf(' ');

      if (lastSpaceIndex > 0) {
        // Truncate at the last complete word
        truncated = truncated.substring(0, lastSpaceIndex);
      }

      return truncated + '...';
    };

    // Split content into sentences or paragraphs
    const sentences = content.split(/[.!?]+/).filter(s => s.trim().length > 0);

    // Find sentences that contain the search query
    const relevantSentences = sentences.filter(sentence =>
      sentence.toLowerCase().includes(query.toLowerCase())
    );

    if (relevantSentences.length > 0) {
      // Take the first relevant sentence and truncate if too long
      let snippet = relevantSentences[0].trim();
      if (snippet.length > 200) {
        snippet = truncateText(snippet, 200);
      }
      return this.highlightText(snippet, query);
    }

    // If no exact matches, find sentences with partial matches
    const queryWords = query.toLowerCase().split(/\s+/).filter(w => w.length > 2);
    const scoredSentences = sentences.map(sentence => {
      const lowerSentence = sentence.toLowerCase();
      let score = 0;
      queryWords.forEach(word => {
        if (lowerSentence.includes(word)) {
          score += word.length; // Longer words get higher scores
        }
      });
      return { sentence, score };
    }).filter(item => item.score > 0);

    if (scoredSentences.length > 0) {
      // Sort by score and take the best match
      scoredSentences.sort((a, b) => b.score - a.score);
      let snippet = scoredSentences[0].sentence.trim();
      if (snippet.length > 200) {
        snippet = truncateText(snippet, 200);
      }
      return this.highlightText(snippet, query);
    }

    // Fallback: take the first sentence and truncate
    if (sentences.length > 0) {
      let snippet = sentences[0].trim();
      if (snippet.length > 200) {
        snippet = truncateText(snippet, 200);
      }
      return snippet;
    }

    return '';
  }

  highlightText(text, query) {
    if (!text) return '';

    const regex = new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
    return text.replace(regex, '<mark>$1</mark>');
  }

  buildTargetUrl(originalTargetUrl, moduleType = null) {
    // console.debug('buildTargetUrl called with:', originalTargetUrl, 'moduleType:', moduleType);

    // If originalTargetUrl is already a full URL or starts with http/https, return as is
    if (originalTargetUrl && (originalTargetUrl.startsWith('http://') || originalTargetUrl.startsWith('https://'))) {
      // console.debug('Full URL detected, returning as is:', originalTargetUrl);
      return originalTargetUrl;
    }

    // If originalTargetUrl is empty or just '#', return current page
    if (!originalTargetUrl || originalTargetUrl === '#') {
      // console.debug('Empty URL, returning current page:', window.location.pathname);
      return window.location.pathname;
    }

    // Get relative current page URL from the meta tag
    const CurrentPageVersionedMeta = document.querySelector('meta[name="page:versioned"]');
    const isCurrentPageVersioned = CurrentPageVersionedMeta && CurrentPageVersionedMeta.content === 'true';
    const isCurrentModulePage = document.querySelector('meta[name="page:module:type"]') !== null ? true : false;
    let relativeCurrentPageURL = document.querySelector('meta[name="page:url:relative"]');
    const isModuleResult = moduleType !== null ? true : false;
    const isEmbeddedModuleResult = moduleType === 'embedded';

    // console.debug('Meta tag found:', relativeCurrentPageURL ? relativeCurrentPageURL.content : 'none');
    // console.debug('Module type:', moduleType);
    // console.debug('Is result for embedded module:', isEmbeddedModuleResult);
    // console.debug('Is a current page versioned:', isCurrentPageVersioned);
    // console.debug('Is a current page a module:', isCurrentModulePage);

    if (relativeCurrentPageURL && relativeCurrentPageURL.content) {
      relativeCurrentPageURL = relativeCurrentPageURL.content;
      // console.debug('Current page relative:', relativeCurrentPageURL);

      // Extract relative path from originalTargetUrl
      let targetModifiedPath = originalTargetUrl;
      // console.debug('Initial target modified path:', targetModifiedPath);

      // Calculate base URL by subtracting page:url:relative from current page URL
      const currentPageUrl = window.location.pathname;
      const match = currentPageUrl.match(/\/(v\d+\.\d+|v\d+|alpha|beta|early-accces|stable|rock-solid|latest)\//);
      const currentPageVersion = match ? match[1] : null;
      const currentPageUrlWithoutVersion = currentPageUrl.replace('/' + currentPageVersion + '/', '/');

      relativeCurrentPageURL = relativeCurrentPageURL.startsWith('./') ?
        relativeCurrentPageURL.substring(2) : relativeCurrentPageURL;

      // console.debug('Current page URL:', currentPageUrl);
      // console.debug('Clean relative path:', relativeCurrentPageURL);

      // Find the base URL
      let baseUrl = currentPageUrlWithoutVersion;
      if (isCurrentModulePage && currentPageUrlWithoutVersion.endsWith(relativeCurrentPageURL)) {
        baseUrl = currentPageUrlWithoutVersion.substring(0, currentPageUrlWithoutVersion.length - relativeCurrentPageURL.length);
        // console.debug('Base URL calculated:', baseUrl);
      } else if (isCurrentPageVersioned && isModuleResult ) {
        baseUrl = '/';
        // console.debug('Base URL calculated (from versioned page to module):', baseUrl);
      } else if (isCurrentPageVersioned && !isModuleResult ) {
        baseUrl = currentPageUrl.substring(0, currentPageUrl.length - relativeCurrentPageURL.length);
        // console.debug('Base URL calculated (versioned page):', baseUrl);
      } else {
        // console.debug('Current URL does not end with relative path, using full URL as base');
      }

      // Construct absolute URL using the base URL
      if (relativeCurrentPageURL) {
        // Remove leading slash from target path to avoid // in the result
        targetModifiedPath = targetModifiedPath.startsWith('/') ?
          targetModifiedPath.substring(1) : targetModifiedPath;

        let result = baseUrl + targetModifiedPath;
        if (currentPageVersion && isCurrentModulePage) {
          // Insert the current version into the URL for modules pages
          result = result.replace(/\/modules\/([^/]+)\//, `/modules/$1/${currentPageVersion}/`);
        }
        // console.debug('Final URL construction:', {
        //   baseUrl,
        //   targetModifiedPath: targetModifiedPath,
        //   result
        // });

        return result;
      }

      // console.debug('No target relative path, returning base URL:', baseUrl);
      return baseUrl;
    }

    // Fallback: return original URL as is
    // console.debug('No meta tag found, returning original URL:', originalTargetUrl);
    return originalTargetUrl;
  }

  showLoading() {
    this.searchResults.style.display = 'flex';
    this.searchResults.innerHTML = `
      <div class="loading">
        <div class="spinner"></div>
        <div class="loading-text">${this.t('loading')}</div>
      </div>
    `;
  }

  hideLoading() {
    // Loading will be replaced by results or message
  }

  showMessage(message) {
    this.searchResults.style.display = 'flex';
    this.searchResults.innerHTML = `<div class="loading">${message}</div>`;
  }

  showNoResults(query) {
    this.searchResults.style.display = 'flex';
    this.searchResults.innerHTML = `
      <div class="no-results">
        ${this.t('noResults', { query: query })}
      </div>
    `;
  }

  showError(message) {
    this.searchResults.style.display = 'flex';
    this.searchResults.innerHTML = `<div class="no-results">${this.t('error')}</div>`;
  }
}

// Initialize search when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
  // Check if there's a data attribute on the search input for custom search index path
  const searchInput = document.getElementById('search-input');
  const searchIndexPath = searchInput?.dataset.searchIndexPath;

  // Create search instance with custom options if specified
  const options = {};
  if (searchIndexPath) {
    options.searchIndexPath = searchIndexPath;
  }

  window.moduleSearch = new ModuleSearch(options);
});

class ModuleSearch {
  constructor(options = {}) {
    this.searchInput = document.getElementById('search-input');
    this.searchResults = document.getElementById('search-results');
    this.searchIndex = null;
    this.searchData = null;
    this.lunrIndex = null;
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

        if (!isClickingOnSearch && !isBlurToSearch) {
          this.searchResults.style.display = 'none';
        }
      }, 150);
    });

    this.searchInput.addEventListener('input', (e) => {
      const query = e.target.value.trim();
      if (query.length > 0) {
        // Show search results when user starts typing
        this.searchResults.style.display = 'flex';
        this.handleSearch(query);
      } else {
        // Hide search results when search is cleared
        this.searchResults.style.display = 'none';
      }
    });

    this.searchInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
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

      // Close search results when clicking outside
      this.searchResults.style.display = 'none';
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

      const response = await fetch(this.options.searchIndexPath);
      if (!response.ok) {
        throw new Error(`Failed to load search index: ${response.status}`);
      }

            this.searchData = await response.json();

      // Refresh language detection before building index
      this.refreshLanguageDetection();

      this.buildLunrIndex();
      this.isDataLoaded = true;
      this.hideLoading();

      // Show message that search index is loaded and ready
      this.showMessage(this.t('ready'));
    } catch (error) {
      console.error('Error loading search index:', error);
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
        this.field('summary', { boost: 5 });
        this.field('content', { boost: 1 });
        this.ref('id');

        // Add documents from the documents array
        if (searchData.documents) {
          searchData.documents.forEach((doc, index) => {
            this.add({
              id: `doc_${index}`,
              title: doc.title || '',
              keywords: doc.keywords || '',
              summary: doc.summary || '',
              content: doc.content || '',
              url: doc.url || '',
              module: doc.module || '',
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
              resName: param.resName || '',
              content: param.content || '',
              url: param.url || '',
              module: param.module || '',
              type: 'parameter'
            });
          });
        }
      });

      console.log('Built search index with Russian multilingual support');
    } else {
      // Use default English language support
      this.lunrIndex = lunr(function() {
        this.field('title', { boost: 10 });
        this.field('keywords', { boost: 8 });
        this.field('summary', { boost: 5 });
        this.field('content', { boost: 1 });
        this.ref('id');

        // Add documents from the documents array
        if (searchData.documents) {
          searchData.documents.forEach((doc, index) => {
            this.add({
              id: `doc_${index}`,
              title: doc.title || '',
              keywords: doc.keywords || '',
              summary: doc.summary || '',
              content: doc.content || '',
              url: doc.url || '',
              module: doc.module || '',
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
              resName: param.resName || '',
              content: param.content || '',
              url: param.url || '',
              module: param.module || '',
              type: 'parameter'
            });
          });
        }
      });

      console.log('Built search index with default English support');
    }
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

      const results = this.lunrIndex.search(query);

      // Apply additional boosting for parameters
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
        if (doc.type === 'parameter' && doc.content && doc.content.includes('resources__prop_name')) {
          boost = 1.5; // Additional boost for parameters with properties
        } else if (doc.type === 'parameter') {
          boost = 1.2; // Moderate boost for parameters
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
          ${this.renderResultGroup(this.currentResults.config, this.lastQuery, 'config')}
        </div>
      `;
    }

    // Display other results
    if (this.currentResults.other.length > 0) {
      resultsHtml += `
        <div class="results-group">
          <div class="results-group-header">${this.t('documentation')}</div>
          ${this.renderResultGroup(this.currentResults.other, this.lastQuery, 'other')}
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
        <a href="${this.buildTargetUrl(doc.url)}" class="result-item">
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

  buildTargetUrl(originalUrl) {
    console.debug('buildTargetUrl called with:', originalUrl);
    
    // If originalUrl is already a full URL or starts with http/https, return as is
    if (originalUrl && (originalUrl.startsWith('http://') || originalUrl.startsWith('https://'))) {
      console.debug('Full URL detected, returning as is:', originalUrl);
      return originalUrl;
    }

    // If originalUrl is empty or just '#', return current page
    if (!originalUrl || originalUrl === '#') {
      console.debug('Empty URL, returning current page:', window.location.pathname);
      return window.location.pathname;
    }


    // Check for meta tag with relative current page URL
    const relativeMeta = document.querySelector('meta[name="page:url:relative"]');
    console.debug('Meta tag found:', relativeMeta ? relativeMeta.content : 'none');
    
    if (relativeMeta && relativeMeta.content) {
      const currentPageRelative = relativeMeta.content;
      console.debug('Current page relative:', currentPageRelative);

      // Extract relative path from originalUrl
      let targetRelativePath = originalUrl;
      console.debug('Initial target relative path:', targetRelativePath);

      // If originalUrl is an absolute path, extract the relative part after the version
      if (originalUrl.startsWith('/')) {
        const urlSegments = originalUrl.split('/').filter(segment => segment);
        console.debug('URL segments:', urlSegments);

        // Find the version segment and extract everything after it
        for (let i = 0; i < urlSegments.length; i++) {
          if (urlSegments[i].match(/^v\d+(\.\d+)*$/)) {
            // Found version segment, take everything after it
            targetRelativePath = urlSegments.slice(i + 1).join('/');
            console.debug('Found version segment at index', i, 'extracted relative path:', targetRelativePath);
            break;
          }
        }
      } else {
        // originalUrl is already relative, use it as is
        console.debug('URL is already relative, using as is:', targetRelativePath);
      }

      // Calculate base URL by subtracting page:url:relative from current page URL
      const currentPageUrl = window.location.pathname;
      const cleanRelativePath = currentPageRelative.startsWith('./') ?
        currentPageRelative.substring(2) : currentPageRelative;
      
      console.debug('Current page URL:', currentPageUrl);
      console.debug('Clean relative path:', cleanRelativePath);
      
      // Find the base URL by removing the relative path from the current URL
      let baseUrl = currentPageUrl;
      if (currentPageUrl.endsWith(cleanRelativePath)) {
        baseUrl = currentPageUrl.substring(0, currentPageUrl.length - cleanRelativePath.length);
        console.debug('Base URL calculated:', baseUrl);
      } else {
        console.debug('Current URL does not end with relative path, using full URL as base');
      }
      
      // Construct absolute URL using the base URL
      if (targetRelativePath) {
        // Remove leading slash from target path to avoid // in the result
        const cleanTargetPath = targetRelativePath.startsWith('/') ? 
          targetRelativePath.substring(1) : targetRelativePath;
        
        // Ensure base URL ends with / for proper concatenation
        const normalizedBaseUrl = baseUrl.endsWith('/') ? baseUrl : baseUrl + '/';
        
        const result = normalizedBaseUrl + cleanTargetPath;
        console.debug('Final URL construction:', {
          baseUrl,
          normalizedBaseUrl,
          targetRelativePath,
          cleanTargetPath,
          result
        });
        
        return result;
      }

      console.debug('No target relative path, returning base URL:', baseUrl);
      return baseUrl;
    }

    // Fallback: return original URL as is
    console.debug('No meta tag found, returning original URL:', originalUrl);
    return originalUrl;
  }

  showLoading() {
    this.searchResults.style.display = 'flex';
    this.searchResults.innerHTML = `<div class="loading">${this.t('loading')}</div>`;
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

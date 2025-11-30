class ModuleSearch {
  constructor(options = {}) {
    this.searchInput = document.getElementById('search-input');
    this.searchResults = document.getElementById('search-results');

    // Check if required DOM elements exist
    if (!this.searchInput) {
      console.error('Search input element not found');
      return;
    }
    if (!this.searchResults) {
      console.error('Search results element not found');
      return;
    }

    // Store the original placeholder from HTML for later restoration
    this.originalPlaceholder = this.searchInput.placeholder;

    this.searchIndex = null;
    this.searchData = null;
    this.lunrIndex = null;
    this.fuseIndex = null;
    this.searchDictionary = [];
    this.availableModules = new Set(); // Store unique module names
    this.lastQuery = '';
    this.pendingQuery = ''; // For storing user input while index is loading
    this.currentResults = {
      modules: [],
      isResourceNameMatch: [],
      nameMatch: [],
      isResourceOther: [],
      parameterOther: [],
      document: []
    };
    this.displayedCounts = {
      isResourceNameMatch: 5,
      nameMatch: 5,
      isResourceOther: 5,
      parameterOther: 5,
      document: 5
    };
    this.isDataLoaded = false;
    this.isLoadingInBackground = false;
    this.searchTimeout = null; // For debouncing search input

    // Configuration options
    this.options = {
      searchIndexPath: '/modules/search-embedded-modules-index.json',
      searchDebounceMs: 300, // Debounce search input by 300ms
      backgroundLoadDelay: 1000, // Delay before starting background loading (1 second)
      searchContext: '', // Search context message to display above ready message
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
        modules: 'Modules',
        showMore: 'Show more',
        loading: 'Loading search index... (you can formulate query, while index is loading)',
        ready: 'What are we looking for?',
        noResults: `Results for "{query}" not found.\nTry different keywords or check your spelling.`,
        error: 'An error occurred during search.',
        showMorePattern: 'Show {count} more',
        modulesMore: '... and +{count} more'
      },
      ru: {
        api: 'API',
        documentation: 'Документация',
        modules: 'Модули',
        showMore: 'Показать еще',
        loading: 'Загрузка поискового индекса... (можно формулировать запрос, пока идет загрузка индекса)',
        ready: 'Что ищем?',
        noResults: "Нет результатов для \"{query}\".\nПопробуйте другие ключевые слова или проверьте правописание.",
        error: 'An error occurred during search.',
        showMorePattern: 'Показать еще {count}',
        modulesMore: '... и ещё {count}'
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

  // Parse search index paths with boost levels
  parseSearchIndexPaths(searchIndexPath) {
    const paths = searchIndexPath.split(',').map(path => path.trim());

    return paths.map(path => {
      // Check if path contains boost level (format: "path:boost")
      const boostMatch = path.match(/^(.+):(\d+(?:\.\d+)?)$/);

      if (boostMatch) {
        return {
          path: boostMatch[1].trim(),
          boost: parseFloat(boostMatch[2])
        };
      } else {
        // Default boost level of 1.0 if not specified
        return {
          path: path,
          boost: 1.0
        };
      }
    });
  }

  async init() {
    this.setupEventListeners();

    // Hide search results by default
    this.searchResults.style.display = 'none';

    // Initialize UI state
    this.updateUIState();

    // Start background loading of search indexes after page is fully loaded
    this.startBackgroundLoading();
  }

  setupEventListeners() {
    // Show search results container when focused
    this.searchInput.addEventListener('focus', () => {
      // Show search results container when focused (even if empty)
      this.searchResults.style.display = 'flex';

      // If data is not loaded and not currently loading, trigger loading
      if (!this.isDataLoaded && !this.isLoadingInBackground) {
        this.showLoading();
        this.searchInput.placeholder = this.t('loading');
        this.loadSearchIndex();
      } else if (this.isDataLoaded) {
        // Data is loaded, check if there's a query in the input
        const query = this.searchInput.value.trim();
        if (query.length > 0) {
          // There's a query, execute the search
          this.searchResults.style.display = 'flex';
          this.handleSearch(query);
        } else {
          // No query, show ready message
          this.updateUIState();
        }
      } else {
        // Data is loading in background, show loading state
        this.updateUIState();
      }
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
          // Restore original HTML placeholder when search is closed
          this.searchInput.placeholder = this.originalPlaceholder;
        } else if (!isClickingOnSearch && !isBlurToSearch) {
          // Even if there are loading/error messages, we should restore the placeholder when closing
          this.searchInput.placeholder = this.originalPlaceholder;
        }
      }, 150);
    });

    this.searchInput.addEventListener('input', (e) => {
      const query = e.target.value.trim();

      // Store user input while index is loading
      if (!this.isDataLoaded) {
        this.pendingQuery = e.target.value; // Store the full value including spaces
        // Show search results container to indicate typing is being captured
        this.searchResults.style.display = 'flex';
        this.showMessage(this.t('loading'));
        return;
      }

      // Clear existing timeout
      if (this.searchTimeout) {
        clearTimeout(this.searchTimeout);
      }

      if (query.length > 0) {
        // Show search results when user starts typing
        this.searchResults.style.display = 'flex';
        // Set placeholder to "ready" when actively searching
        if (this.isDataLoaded) {
          this.searchInput.placeholder = this.t('ready');
        }
        // Debounce the search to prevent excessive calls
        this.searchTimeout = setTimeout(() => {
          this.handleSearch(query);
        }, this.options.searchDebounceMs);
      } else {
        // Input is cleared - hide search results and restore HTML placeholder
        this.searchResults.style.display = 'none';
        this.searchInput.placeholder = this.originalPlaceholder;
      }
    });

    this.searchInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        const query = e.target.value.trim();

        // Store user input while index is loading
        if (!this.isDataLoaded) {
          this.pendingQuery = e.target.value; // Store the full value including spaces
          return;
        }

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
      if (!isClickOnSearch) {
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

  startBackgroundLoading() {
    // Don't start if already loaded or currently loading
    if (this.isDataLoaded || this.isLoadingInBackground) {
      return;
    }

    // Wait for page to be fully loaded before starting background loading
    if (document.readyState === 'complete') {
      // Page is already loaded, start background loading after a delay
      setTimeout(() => {
        this.loadSearchIndexInBackground();
      }, this.options.backgroundLoadDelay);
    } else {
      // Wait for page to finish loading
      window.addEventListener('load', () => {
        setTimeout(() => {
          this.loadSearchIndexInBackground();
        }, this.options.backgroundLoadDelay);
      });
    }
  }

  async loadSearchIndexInBackground() {
    // Don't load if already loaded or currently loading
    if (this.isDataLoaded || this.isLoadingInBackground) {
      return;
    }

    this.isLoadingInBackground = true;

    try {
      await this.loadSearchIndex();
    } catch (error) {
      console.warn('Background loading of search index failed:', error);
    } finally {
      this.isLoadingInBackground = false;
    }
  }

  async loadSearchIndex() {
    if (this.isDataLoaded) {
      return; // Already loaded
    }

    try {
      // Only show loading UI if not loading in background
      if (!this.isLoadingInBackground) {
        this.showLoading();
      }

      // Parse search index paths with boost levels
      const indexConfigs = this.parseSearchIndexPaths(this.options.searchIndexPath);

      if (indexConfigs.length === 1) {
        // Single index file
        const config = indexConfigs[0];
        const response = await fetch(config.path);
        if (!response.ok) {
          throw new Error(`Failed to load search index: ${response.status}`);
        }
        this.searchData = await response.json();
        this.searchData.boostLevel = config.boost;
      } else {
        // Multiple index files - load and merge them
        console.log(`Loading ${indexConfigs.length} search index files:`, indexConfigs.map(c => `${c.path} (boost: ${c.boost})`));

        const responses = await Promise.all(
          indexConfigs.map(async (config) => {
            try {
              const response = await fetch(config.path);
              if (!response.ok) {
                console.warn(`Failed to load search index: ${config.path} (${response.status})`);
                return { documents: [], parameters: [], boost: config.boost };
              }
              const data = await response.json();
              data.boost = config.boost;
              return data;
            } catch (error) {
              console.warn(`Error loading search index: ${config.path}`, error);
              return { documents: [], parameters: [], boost: config.boost };
            }
          })
        );

        // Merge all search indexes with boost information
        this.searchData = {
          documents: [],
          parameters: [],
          indexBoosts: {} // Store boost levels for each index
        };

        responses.forEach((indexData, index) => {
          if (indexData && indexData.documents) {
            // Add boost information to each document
            const boostedDocuments = indexData.documents.map(doc => ({
              ...doc,
              _indexBoost: indexData.boost,
              _indexSource: indexConfigs[index].path
            }));
            this.searchData.documents = this.searchData.documents.concat(boostedDocuments);
            console.log(`Added ${indexData.documents.length} documents from ${indexConfigs[index].path}`);
          }
          if (indexData && indexData.parameters) {
            // Add boost information to each parameter
            const boostedParameters = indexData.parameters.map(param => ({
              ...param,
              _indexBoost: indexData.boost,
              _indexSource: indexConfigs[index].path
            }));
            this.searchData.parameters = this.searchData.parameters.concat(boostedParameters);
            console.log(`Added ${indexData.parameters.length} parameters from ${indexConfigs[index].path}`);
          }
          // Store boost level for this index
          this.searchData.indexBoosts[indexConfigs[index].path] = indexData.boost;
        });

        console.log(`Merged search data: ${this.searchData.documents.length} documents, ${this.searchData.parameters.length} parameters`);
      }

      // Refresh language detection before building index
      this.refreshLanguageDetection();

      this.buildLunrIndex();
      this.buildSearchDictionary();
      this.buildFuseIndex();
      this.extractAvailableModules();
      this.isDataLoaded = true;

      // Only hide loading UI if not loading in background
      if (!this.isLoadingInBackground) {
        this.hideLoading();
      }

      // Update UI state (including placeholder)
      this.updateUIState();

      // Only focus and show UI if not loading in background
      if (!this.isLoadingInBackground) {
        // Keep focus on search input after loading
        this.searchInput.focus();

        // Execute search with pending query if user was typing while loading
        if (this.pendingQuery && this.pendingQuery.trim().length > 0) {
          // Update the input value to match what the user typed
          this.searchInput.value = this.pendingQuery;
          this.searchResults.style.display = 'flex';
          this.handleSearch(this.pendingQuery.trim());
          console.log('Executed search with pending query after on-demand loading:', this.pendingQuery);
          this.pendingQuery = ''; // Clear pending query
        } else {
          // Show message that search index is loaded and ready
          this.showMessage(this.t('ready'));
        }
      } else {
        // Background loading completed
        // Update UI to reflect that data is now loaded
        this.updateUIState();

        // Execute search with pending query if user was typing while loading
        if (this.pendingQuery && this.pendingQuery.trim().length > 0) {
          // Update the input value to match what the user typed
          this.searchInput.value = this.pendingQuery;
          this.searchResults.style.display = 'flex';
          this.handleSearch(this.pendingQuery.trim());
          console.log('Executed search with pending query after background loading:', this.pendingQuery);
        }

        // Clear pending query after processing
        this.pendingQuery = '';
      }
    } catch (error) {
      console.error('Error loading search index:', error);
      // Update UI state (including placeholder)
      this.updateUIState();

      // Only show error UI if not loading in background
      if (!this.isLoadingInBackground) {
        // Keep focus on search input after error
        this.searchInput.focus();
        this.showError('Failed to load search index. Please try again later.');
      }
    }
  }

  buildLunrIndex() {
    const searchData = this.searchData;
    const useRussianSupport = this.currentLang === 'ru' && typeof lunr.multiLanguage !== 'undefined';

    // Use multilingual support for Russian, default for English
    this.lunrIndex = lunr(function() {
      // Configure language support
      if (useRussianSupport) {
        this.use(lunr.multiLanguage('en', 'ru'));
      }

      // Configure fields
      this.field('title', { boost: 10 });
      this.field('keywords', { boost: 8 });
      this.field('module', { boost: 6 });
      this.field('summary', { boost: 3 });
      this.field('content', { boost: 1 });
      this.ref('id');

      // Add documents from the documents array
      let docCounter = 0;
      if (searchData.documents) {
        searchData.documents.forEach((doc) => {
          const docData = {
            id: `doc_${docCounter}`,
            title: doc.title || '',
            keywords: doc.keywords || '',
            module: doc.module || '',
            summary: doc.summary || '',
            content: doc.content || '',
            url: doc.url || '',
            type: 'document'
          };

          // Add moduletype only for Russian support (backward compatibility)
          if (useRussianSupport && doc.moduletype) {
            docData.moduletype = doc.moduletype;
          }

          this.add(docData);
          docCounter++;
        });
      }

      // Add parameters from the parameters array
      let paramCounter = 0;
      if (searchData.parameters) {
        searchData.parameters.forEach((param) => {
          const paramData = {
            id: `param_${paramCounter}`,
            title: param.name || '',
            keywords: param.keywords || '',
            module: param.module || '',
            resName: param.resName || '',
            content: param.content || '',
            url: param.url || '',
            type: 'parameter'
          };

          // Add moduletype only for Russian support (backward compatibility)
          if (useRussianSupport && param.moduletype) {
            paramData.moduletype = param.moduletype;
          }

          this.add(paramData);
          paramCounter++;
        });
      }
    });
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
      threshold: 0.4, // Higher threshold = more lenient matching (0.0 = exact, 1.0 = match anything)
      distance: 100,  // Maximum distance for fuzzy matching
      includeScore: true,
      minMatchCharLength: 2,
      // Better support for Cyrillic characters
      ignoreLocation: true,
      findAllMatches: false,
      useExtendedSearch: false
    });

    console.log('Built Fuse.js index for fuzzy search');
  }

  extractAvailableModules() {
    // Extract all unique module names from documents and parameters
    this.availableModules.clear();

    // Extract from documents
    if (this.searchData.documents) {
      this.searchData.documents.forEach(doc => {
        if (doc.module && doc.module.trim()) {
          this.availableModules.add(doc.module.trim());
        }
      });
    }

    // Extract from parameters
    if (this.searchData.parameters) {
      this.searchData.parameters.forEach(param => {
        if (param.module && param.module.trim()) {
          this.availableModules.add(param.module.trim());
        }
      });
    }

    console.log(`Extracted ${this.availableModules.size} unique modules`);
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

  getModulePageResults(query) {
    const results = [];
    const queryLower = query.toLowerCase().trim();

    // Check if query matches any module name
    this.availableModules.forEach(moduleName => {
      const moduleLower = moduleName.toLowerCase();

      // Check for exact match or if module name contains the query
      if (moduleLower === queryLower || moduleLower.includes(queryLower)) {
        // Create a synthetic result for the module page
        // Use a special ID format to identify module page results
        // Special case for "global" module
        const moduleUrl = moduleName === 'global'
          ? '/products/kubernetes-platform/documentation/v1/reference/api/global.html'
          : `/modules/${moduleName}/`;

        const modulePageResult = {
          ref: `module_page_${moduleName}`,
          score: moduleLower === queryLower ? 1000 : 500, // Higher score for exact matches
          _isModulePage: true,
          _moduleName: moduleName,
          _moduleUrl: moduleUrl
        };
        results.push(modulePageResult);
      }
    });

    // Sort by score (exact matches first)
    results.sort((a, b) => b.score - a.score);

    return results;
  }

  // Check if query looks like a URL and sanitize it for search
  sanitizeQueryForSearch(query) {
    // Check if the query looks like a URL
    const urlPattern = /^https?:\/\/[^\s]+$/i;
    if (urlPattern.test(query)) {
      // Extract domain and path from URL for searching
      try {
        const url = new URL(query);
        // Extract meaningful parts: domain and path segments
        const domain = url.hostname.replace(/^www\./, ''); // Remove www prefix
        const pathSegments = url.pathname.split('/').filter(segment => segment.length > 0);

        // Create search terms from URL parts
        const searchTerms = [domain, ...pathSegments].join(' ');
        console.log(`URL detected, sanitized query: "${query}" -> "${searchTerms}"`);
        return searchTerms;
      } catch (e) {
        // If URL parsing fails, just remove the protocol and special characters
        const sanitized = query.replace(/^https?:\/\//, '').replace(/[^\w\s-]/g, ' ').trim();
        console.log(`URL parsing failed, basic sanitization: "${query}" -> "${sanitized}"`);
        return sanitized;
      }
    }

    // Apply comprehensive sanitization for all Lunr special operators and patterns
    let sanitized = query;
    let hasChanges = false;

    // Handle field patterns like "field:value" or queries starting with colon like ":version"
    if (/^[a-zA-Z]*:/.test(sanitized)) {
      sanitized = sanitized.replace(/:/g, ' ');
      hasChanges = true;
    }

    // Handle Lunr PRESENCE operator (--)
    if (sanitized.includes('--')) {
      sanitized = sanitized.replace(/--/g, ' ');
      hasChanges = true;
    }

    // Handle other Lunr operators (+ and - at the beginning of words)
    const lunrOperatorPattern = /(\s|^)[+\-](\w+)/g;
    if (lunrOperatorPattern.test(sanitized)) {
      sanitized = sanitized.replace(lunrOperatorPattern, '$1$2');
      hasChanges = true;
    }

    if (hasChanges) {
      sanitized = sanitized.trim();
      // console.log(`Lunr operators detected, sanitized: "${query}" -> "${sanitized}"`);
      return sanitized;
    }

    return query;
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
      // Sanitize the query to handle URLs and other problematic patterns
      const sanitizedQuery = this.sanitizeQueryForSearch(query);

      this.lastQuery = query; // Keep original query for display
      this.resetPagination();

      // Clear any existing fuzzy search messages
      this.clearFuzzySearchMessages();

      // First try exact search with sanitized query
      let results = [];
      let highlightQuery = sanitizedQuery; // Use sanitized query for highlighting

      try {
        results = this.lunrIndex.search(sanitizedQuery);
      } catch (error) {
        console.warn('Lunr search error with sanitized query:', error);
        // If sanitized query still fails, try a more aggressive sanitization
        const fallbackQuery = sanitizedQuery.replace(/[^\w\s-]/g, ' ').replace(/\s+/g, ' ').trim();
        if (fallbackQuery !== sanitizedQuery) {
          try {
            results = this.lunrIndex.search(fallbackQuery);
            highlightQuery = fallbackQuery;
            console.log(`Fallback search successful with: "${fallbackQuery}"`);
          } catch (fallbackError) {
            console.error('Fallback search also failed:', fallbackError);
            this.showError('Search query contains invalid characters. Please try a different search term.');
            return;
          }
        } else {
          this.showError('Search query contains invalid characters. Please try a different search term.');
          return;
        }
      }

      // If no results and fuzzy search is available, try fuzzy search
      if (results.length === 0 && this.fuseIndex) {
        const fuzzySuggestions = this.getFuzzySuggestions(sanitizedQuery);

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
        const fuzzySuggestions = this.getFuzzySuggestions(sanitizedQuery);
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

      // Apply additional boosting for parameters, module name matches, and index boost levels
      let boostedResults = results.map(result => {
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

        // Apply index boost level if available
        if (doc._indexBoost) {
          boost *= doc._indexBoost;
        }

        // Check if the search query matches the module name
        const queryLower = sanitizedQuery.toLowerCase();
        const moduleLower = (doc.module || '').toLowerCase();

        if (moduleLower && moduleLower.includes(queryLower)) {
          boost *= 1.8; // Strong boost for module name matches
        }

        // Check for parameter field matches with specific priority order
        if (doc.type === 'parameter') {
          const nameLower = (doc.name || '').toLowerCase();
          const keywordsLower = (doc.keywords && typeof doc.keywords === 'string') ? doc.keywords.toLowerCase() : '';
          const contentLower = (doc.content || '').toLowerCase();

          // Priority 1: Name field matches (highest priority)
          if (nameLower) {
            if (nameLower === queryLower) {
              boost *= 4.0; // Very high boost for exact name matches
            } else if (nameLower.includes(queryLower)) {
              boost *= 3.5; // High boost for partial name matches
            }
          }

          // Priority 2: Keywords field matches
          if (keywordsLower && keywordsLower.includes(queryLower)) {
            boost *= 2.0; // Moderate boost for keyword matches
          }

          // Priority 3: Content field matches (lowest priority for parameters)
          if (contentLower && contentLower.includes(queryLower)) {
            boost *= 1.2; // Low boost for content matches
          }
        } else {
          // For non-parameters (documents), use document field priority order
          const titleLower = (doc.title || '').toLowerCase();
          const keywordsLower = (doc.keywords && typeof doc.keywords === 'string') ? doc.keywords.toLowerCase() : '';
          const contentLower = (doc.content || '').toLowerCase();

          // Priority 1: Title field matches (highest priority)
          if (titleLower) {
            if (titleLower === queryLower) {
              boost *= 4.0; // Very high boost for exact title matches
            } else if (titleLower.includes(queryLower)) {
              boost *= 3.5; // High boost for partial title matches
            }
          }

          // Priority 2: Keywords field matches
          if (keywordsLower && keywordsLower.includes(queryLower)) {
            boost *= 2.0; // Moderate boost for keyword matches
          }

          // Priority 3: Content field matches (lowest priority for documents)
          if (contentLower && contentLower.includes(queryLower)) {
            boost *= 1.2; // Low boost for content matches
          }
        }

        // Apply existing parameter boosting logic
        if (doc.type === 'parameter' && doc.content && doc.content.includes('resources__prop_name')) {
          boost *= 1.5; // Additional boost for parameters with properties
        } else if (doc.type === 'parameter') {
          boost *= 1.2; // Moderate boost for parameters
        }

        // Apply additional boost for isResource parameters
        if (doc.type === 'parameter' && doc.isResource === "true") {
          boost *= 2.0; // High boost for isResource parameters to prioritize them
        }

        return {
          ...result,
          score: result.score * boost
        };
      });

      // Sort by boosted score
      boostedResults.sort((a, b) => b.score - a.score);

      // Check if query matches any module name and add module page results
      const modulePageResults = this.getModulePageResults(sanitizedQuery);
      if (modulePageResults.length > 0) {
        // Add module page results with high priority (insert at the beginning)
        boostedResults = modulePageResults.concat(boostedResults);
      }

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
    const modulesResults = [];
    const isResourceNameMatchResults = [];
    const nameMatchResults = [];
    const isResourceOtherResults = [];
    const parameterOtherResults = [];
    const documentResults = [];

    results.forEach(result => {
      const docId = result.ref;

      // Handle module page results
      if (result._isModulePage) {
        // Module pages go to modules group
        modulesResults.push(result);
        return;
      }

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
        // Check for name matches first
        const nameLower = (doc.name || doc.title || '').toLowerCase();
        const queryLower = this.lastQuery.toLowerCase();
        const hasNameMatch = nameLower && (nameLower === queryLower || nameLower.includes(queryLower));

        if (doc.type === 'parameter') {
          // Check if this parameter has isResource: "true"
          if (doc.isResource === "true") {
            if (hasNameMatch) {
              isResourceNameMatchResults.push(result);
            } else {
              isResourceOtherResults.push(result);
            }
          } else {
            if (hasNameMatch) {
              nameMatchResults.push(result);
            } else {
              parameterOtherResults.push(result);
            }
          }
        } else {
          // Documents always go to document group
          documentResults.push(result);
        }
      }
    });

    return {
      modules: modulesResults,
      isResourceNameMatch: isResourceNameMatchResults,
      nameMatch: nameMatchResults,
      isResourceOther: isResourceOtherResults,
      parameterOther: parameterOtherResults,
      document: documentResults
    };
  }

  displayResults() {
    // Dynamically check all keys in currentResults, so new groups are automatically included
    if (Object.values(this.currentResults).every(arr => arr.length === 0)) {
      this.showNoResults(this.lastQuery);
      return;
    }

    let resultsHtml = '';

    // Display Modules as a row at the top
    if (this.currentResults.modules.length > 0) {
      resultsHtml += this.renderModulesRow(this.currentResults.modules, this.currentHighlightQuery || this.lastQuery);
    }

    // Display API results in priority order
    if (this.currentResults.isResourceNameMatch.length > 0 || this.currentResults.nameMatch.length > 0 || this.currentResults.isResourceOther.length > 0 || this.currentResults.parameterOther.length > 0) {
      resultsHtml += `
        <div class="results-group">
          <div class="results-group-header">${this.t('api')}</div>
          ${this.currentResults.isResourceNameMatch.length > 0 ? this.renderResultGroup(this.currentResults.isResourceNameMatch, this.currentHighlightQuery || this.lastQuery, 'isResourceNameMatch') : ''}
          ${this.currentResults.nameMatch.length > 0 ? this.renderResultGroup(this.currentResults.nameMatch, this.currentHighlightQuery || this.lastQuery, 'nameMatch') : ''}
          ${this.currentResults.isResourceOther.length > 0 ? this.renderResultGroup(this.currentResults.isResourceOther, this.currentHighlightQuery || this.lastQuery, 'isResourceOther') : ''}
          ${this.currentResults.parameterOther.length > 0 ? this.renderResultGroup(this.currentResults.parameterOther, this.currentHighlightQuery || this.lastQuery, 'parameterOther') : ''}
        </div>
      `;
    }

    // Display documentation results (only from documents array)
    if (this.currentResults.document.length > 0) {
      resultsHtml += `
        <div class="results-group">
          <div class="results-group-header">${this.t('documentation')}</div>
          ${this.renderResultGroup(this.currentResults.document, this.currentHighlightQuery || this.lastQuery, 'document')}
        </div>
      `;
    }

    this.searchResults.innerHTML = resultsHtml;
  }

  renderModulesRow(results, query) {
    const moduleBadges = results.map(result => {
      if (result._isModulePage) {
        const moduleName = result._moduleName;
        const moduleUrl = result._moduleUrl;
        return `<a href="${moduleUrl}" class="result-module">${moduleName}</a>`;
      }
      return '';
    }).filter(badge => badge !== '');

    if (moduleBadges.length === 0) {
      return '';
    }

    // Limit to 14 modules, add count badge if more
    const maxModules = 14;
    const displayBadges = moduleBadges.slice(0, maxModules);
    const hasMore = moduleBadges.length > maxModules;
    const remainingCount = hasMore ? moduleBadges.length - maxModules : 0;

    let html = `<div class="modules-row">
      <span class="modules-label">${this.t('modules')}:</span> `;
    html += displayBadges.join('');
    if (hasMore) {
      html += `<span class="modules-more">${this.t('modulesMore', { count: remainingCount })}</span>`;
    }
    html += '</div>';

    return html;
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

      let title, module, description;

      if (groupType === 'isResourceNameMatch' || groupType === 'nameMatch' || groupType === 'isResourceOther' || groupType === 'parameterOther') {
        // For configuration results (parameters) and isResource parameters
        title = this.highlightText(doc.name || '', query);
        module = doc.module ? `<div class="result-module">${doc.module}</div>` : '';
        if (doc.resName != doc.name) {
          module += doc.resName ? `<div class="result-module">${doc.resName}</div>` : '';
        }
        description = this.highlightText(this.getRelevantContentSnippet(doc.content || '', query) || '', query);
      } else {
        // For other documentation
        title = this.highlightText(doc.title || '', query);
        module = doc.module ? `<div class="result-module">${doc.module}</div>` : '';
        description = this.highlightText(this.getRelevantContentSnippet(doc.content || '', query) || '', query);
      }

      html += `
        <a href="${this.buildTargetUrl(doc.url, doc.moduletype, doc.module)}" class="result-item">
          <div class="result-title">${title}</div>
          ${module}
          <div class="result-description">${description}</div>
        </a>
      `;
    });

    return html;
  }

  loadMore(groupType) {
    if (groupType === 'isResourceNameMatch' || groupType === 'nameMatch' || groupType === 'isResourceOther' || groupType === 'parameterOther' || groupType === 'document') {
      this.displayedCounts[groupType] += 5;
      this.displayResults();
    }
  }

  resetPagination() {
    this.displayedCounts = {
      isResourceNameMatch: 5,
      nameMatch: 5,
      isResourceOther: 5,
      parameterOther: 5,
      document: 5
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

  buildTargetUrl(originalTargetUrl, moduleType = null, moduleName = null) {
    // console.debug('buildTargetUrl called with:', originalTargetUrl, 'moduleType:', moduleType, 'moduleName:', moduleName);

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
    const isModuleResult = (moduleType !== null && moduleName !== null && moduleName !== 'global');
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
      const match = currentPageUrl.match(/\/(v\d+\.\d+|v\d+|alpha|beta|early-access|stable|rock-solid|latest)\//);
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
        console.debug('Current URL does not end with relative path, using full URL as base');
      }

      // Construct absolute URL using the base URL
      if (relativeCurrentPageURL || relativeCurrentPageURL === '') {
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
        <div class="loading-text">${this.t('loading')}</div>
        <div class="spinner-small"></div>
      </div>
    `;
  }

  hideLoading() {
    // Loading will be replaced by results or message
  }

  showMessage(message) {
    this.searchResults.style.display = 'flex';

    // If this is the ready message and we have a search context, show the context message
    if (message === this.t('ready') && this.options.searchContext) {
      this.searchResults.innerHTML = `<div class="loading">${this.options.searchContext}</div>`;
    } else {
      this.searchResults.innerHTML = `<div class="loading">${message}</div>`;
    }
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
    this.searchResults.innerHTML = `<div class="no-results">${message}</div>`;
  }

  // Check current state and update UI accordingly
  updateUIState() {
    if (this.isDataLoaded) {
      // Only set placeholder to "ready" when search results are visible (user is actively searching)
      if (this.searchResults.style.display === 'flex') {
        this.searchInput.placeholder = this.t('ready');
        this.showMessage(this.t('ready'));
      }
      // Don't change placeholder when search results are hidden (let HTML placeholder show)
    } else if (this.isLoadingInBackground) {
      this.searchInput.placeholder = this.t('loading');
      if (this.searchResults.style.display === 'flex') {
        this.showLoading();
      }
    }
    // Don't set placeholder when data is not loaded and not loading (let HTML placeholder show)
  }
}

// Initialize search when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
  // Check if there's a data attribute on the search input for custom search index path
  const searchInput = document.getElementById('search-input');
  const searchIndexPath = searchInput?.dataset.searchIndexPath;
  const searchContext = searchInput?.dataset.searchContext;

  // Create search instance with custom options if specified
  const options = {};
  if (searchIndexPath) {
    options.searchIndexPath = searchIndexPath;
  }
  if (searchContext) {
    options.searchContext = searchContext;
  }

  window.moduleSearch = new ModuleSearch(options);
});

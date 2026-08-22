// Rewrites applied to every query before it reaches Lunr, in order. The search box
// takes plain text: the only operator left is a trailing "*" on a term of at least
// three characters. Measured on the site index (562 documents, 6592 parameters):
// "ingress~5" matched 2464 pages in 76 ms, a leading "*gress" cost 111 ms against
// 2 ms for "ingres*", "*" alone meant the whole corpus (7154 hits, 993 ms), and
// "content:nginx" silently narrowed 218 hits to 107 with nothing in the UI to show it.
//
// This table lives here only: the main thread sanitizes every query before it reaches
// the worker (see searchWithWorker()) and sanitizes the synonym map it sends on INIT,
// so search-v3-worker.js never sees raw user input. The rules are still idempotent,
// because buildPhraseQuery() re-adds "+" to build conjunctions after this pass.
const LUNR_SYNTAX_RULES = [
  // Presence: "+ingress", "-nginx", "install --dry-run". The old rule captured (\w+),
  // and \w is ASCII only, so "+кластер" stayed a working operator while "+ingress"
  // did not - the lookahead makes this independent of the keyboard layout.
  [/(^|\s)[+-]+(?=[^\s+-])/gu, '$1'],
  // A dangling "+" or "-" is a parse error on its own.
  [/(^|\s)[+-]+(?=\s|$)/gu, '$1'],
  // Fuzzy matching: too blunt (an edit distance of 5 matches almost anything) and it
  // throws on a non-numeric operand.
  [/~\d*/gu, ' '],
  // Boost: relevance is the job of the field boosts in buildLunrIndex().
  [/\^\d*/gu, ' '],
  // Wildcards: keep one trailing "*", drop the expensive forms. A star inside a term
  // acts as a separator, mirroring how the colon is treated below.
  [/\*+(?=[^\s*])/gu, ' '],
  [/(^|\s)([^\s*]{0,2})\*+/gu, '$1$2'],
  [/\*{2,}/gu, '*'],
  // Field scoping: "kind: configmap" has to search for words. The index holds "kind",
  // not "kind:" - lunr.trimmer strips it - so a space reproduces the indexed tokens.
  // The pair itself stays meaningful, see isKeyValueQuery().
  [/:/gu, ' ']
];

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
    // Results shown per rendered block initially and added by each "show more"
    // click. The four API subgroups share the 'api' counter, see displayResults().
    this.pageSize = 5;
    this.displayedCounts = {
      api: this.pageSize,
      document: this.pageSize
    };
    this.isDataLoaded = false;
    this.isLoadingInBackground = false;
    this.searchIndexLoadPromise = null; // Shared by concurrent loadSearchIndex() callers
    this.searchTimeout = null; // For debouncing search input
    this.useSearchWorker = typeof Worker !== 'undefined';
    this.workerCompatibilityChecked = false;
    this.searchWorker = null;
    this.workerInitialized = false;
    this.workerInitResolve = null;
    this.workerInitReject = null;
    this.workerRequestCounter = 0;
    this.workerPendingRequests = new Map();
    this.indexedDBAvailable = false; // Flag to track IndexedDB availability
    this.dbName = 'ModuleSearchDB';
    this.dbVersion = 2; // Bump to drop caches built before the searchBoost field existed
    this.storeName = 'searchIndexes';
    this.cacheExpirationMs = 3600000; // 1 hour in milliseconds

    // Configuration options
    this.options = {
      searchIndexPath: '/modules/search-embedded-modules-index.json',
      searchDebounceMs: 300, // Debounce search input by 300ms
      backgroundLoadDelay: 1000, // Delay before starting background loading (1 second)
      searchContext: '', // Search context message to display above ready message
      workerPath: '/assets/js/search-v3-worker.js',
      // Groups of equivalent terms: every member expands to all the others, so
      // links work in both directions without hand-written reverse entries.
      // This is the single source of truth — the worker gets the derived map on INIT.
      synonymGroups: [
        ['moduleupdatepolicy', 'update policy', 'module update policy', 'политика обновления'],
        ['dexprovider', 'dex providers', 'провайдеры аутентификации'],
        ['modulepulloverride', 'переопределение'],
        ['release.deckhouse.io/approved', 'ручное подтверждение обновлений'],
        ['siem', 'Security Information and Event Management', 'kuma', 'kesl', 'kaspersky container security', 'Kaspersky Unified Monitoring and Analysis Platform']
      ],
      // One-way overrides, if some term must expand without the reverse link:
      // { 'what user types': ['extra query', ...] }. Merged on top of the groups.
      synonyms: {},
      ...options
    };

    // Initialize i18n
    this.initI18n();

    // Initialize IndexedDB
    this.initIndexedDB().then(() => {
      this.init();
    }).catch(() => {
      // If IndexedDB fails, continue with fallback method
      this.init();
    });
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

  // Reads the cache-busting version from the loaded search-v3.js tag.
  getCurrentScriptVersion() {
    const matchingScript = Array.from(document.scripts).find((script) =>
      script.src && script.src.includes('/assets/js/search-v3.js')
    );
    if (!matchingScript) {
      return '';
    }
    const url = new URL(matchingScript.src, window.location.origin);
    return url.searchParams.get('v') || '';
  }

  // Builds the worker URL and keeps it on the same asset version as main script.
  getWorkerUrl() {
    const version = this.getCurrentScriptVersion();
    if (!version) {
      return this.options.workerPath;
    }
    return `${this.options.workerPath}?v=${encodeURIComponent(version)}`;
  }

  // Compatibility gate: verifies Worker APIs and basic execution.
  async canUseSearchWorker() {
    if (this.workerCompatibilityChecked) {
      return this.useSearchWorker;
    }
    this.workerCompatibilityChecked = true;

    if (typeof Worker === 'undefined' || typeof URL === 'undefined' || typeof Blob === 'undefined') {
      this.useSearchWorker = false;
      return false;
    }

    let testUrl = null;
    try {
      const testBlob = new Blob(
        ["self.onmessage=function(){self.postMessage('ok');};"],
        { type: 'application/javascript' }
      );
      testUrl = URL.createObjectURL(testBlob);

      const isSupported = await new Promise((resolve) => {
        const testWorker = new Worker(testUrl);
        const timer = setTimeout(() => {
          testWorker.terminate();
          resolve(false);
        }, 1500);

        testWorker.onmessage = () => {
          clearTimeout(timer);
          testWorker.terminate();
          resolve(true);
        };

        testWorker.onerror = () => {
          clearTimeout(timer);
          testWorker.terminate();
          resolve(false);
        };

        testWorker.postMessage('ping');
      });

      this.useSearchWorker = isSupported;
      return isSupported;
    } catch (error) {
      this.useSearchWorker = false;
      return false;
    } finally {
      if (testUrl) {
        URL.revokeObjectURL(testUrl);
      }
    }
  }

  // Creates the dedicated worker and wires message/error handlers once.
  async initSearchWorker() {
    if (!this.useSearchWorker) {
      return false;
    }

    if (this.searchWorker) {
      return true;
    }

    try {
      this.searchWorker = new Worker(this.getWorkerUrl());

      this.searchWorker.onmessage = (event) => {
        const { type, payload } = event.data || {};
        if (type === 'READY') {
          this.workerInitialized = true;
          this.availableModules = new Set(payload.availableModules || []);
          if (this.workerInitResolve) {
            this.workerInitResolve(true);
          }
          this.workerInitResolve = null;
          this.workerInitReject = null;
          return;
        }

        if (type === 'SEARCH_RESULT') {
          const requestId = payload.requestId;
          const pending = this.workerPendingRequests.get(requestId);
          if (pending) {
            this.workerPendingRequests.delete(requestId);
            pending.resolve(payload);
          }
          return;
        }

        if (type === 'ERROR') {
          const requestId = payload && payload.requestId;
          if (requestId && this.workerPendingRequests.has(requestId)) {
            const pending = this.workerPendingRequests.get(requestId);
            this.workerPendingRequests.delete(requestId);
            pending.reject(new Error(payload.message || 'Worker search error'));
          } else if (this.workerInitReject) {
            this.workerInitReject(new Error(payload.message || 'Worker initialization failed'));
            this.workerInitResolve = null;
            this.workerInitReject = null;
          }
        }
      };

      this.searchWorker.onerror = (event) => {
        console.warn('Search worker error, falling back to main thread:', event.message);
        this.useSearchWorker = false;
        this.workerInitialized = false;
        if (this.searchWorker) {
          this.searchWorker.terminate();
          this.searchWorker = null;
        }
        if (this.workerInitReject) {
          this.workerInitReject(new Error(event.message || 'Search worker error'));
        }
        this.workerInitResolve = null;
        this.workerInitReject = null;
      };
    } catch (error) {
      console.warn('Failed to create search worker, falling back to main thread:', error);
      this.useSearchWorker = false;
      if (this.workerInitReject) {
        this.workerInitReject(error);
      }
      this.workerInitResolve = null;
      this.workerInitReject = null;
      return false;
    }

    return true;
  }

  // Initializes search in worker first, then falls back to in-thread indexing.
  async initializeSearchEngine() {
    if (this.useSearchWorker) {
      await this.canUseSearchWorker();
    }

    if (this.useSearchWorker) {
      try {
        const workerCreated = await this.initSearchWorker();
        if (!workerCreated) {
          throw new Error('Worker was not created');
        }

        this.workerInitialized = false;
        const readyPromise = new Promise((resolve, reject) => {
          const timeoutId = setTimeout(() => {
            if (!this.workerInitialized) {
              reject(new Error('Search worker initialization timeout'));
            }
          }, 20000);
          this.workerInitResolve = (value) => {
            clearTimeout(timeoutId);
            resolve(value);
          };
          this.workerInitReject = (error) => {
            clearTimeout(timeoutId);
            reject(error);
          };
        });

        this.searchWorker.postMessage({
          type: 'INIT',
          payload: {
            searchData: this.searchData,
            currentLang: this.currentLang,
            // Derived map, not the raw options: the worker has no synonym data of its own.
            synonyms: this.getNormalizedSynonyms()
          }
        });

        await readyPromise;
        return;
      } catch (error) {
        console.warn('Worker init failed, falling back to main thread:', error);
        this.useSearchWorker = false;
        this.workerInitialized = false;
        if (this.searchWorker) {
          this.searchWorker.terminate();
          this.searchWorker = null;
        }
      }
    }

    this.buildLunrIndex();
    this.buildSearchDictionary();
    this.buildFuseIndex();
    this.extractAvailableModules();
  }

  // Sends a search request to worker and resolves by requestId. The query is sanitized
  // here, not in the worker: query syntax is owned by the main thread, which passes the
  // result of that reading along - requireAllWords for a "key: value" pair.
  searchWithWorker(query, requireAllWords) {
    return new Promise((resolve, reject) => {
      if (!this.searchWorker || !this.workerInitialized) {
        reject(new Error('Search worker is not ready'));
        return;
      }

      const requestId = ++this.workerRequestCounter;
      this.workerPendingRequests.set(requestId, { resolve, reject });

      this.searchWorker.postMessage({
        type: 'SEARCH',
        payload: {
          requestId,
          query,
          requireAllWords: requireAllWords === true
        }
      });
    });
  }

  // Initialize IndexedDB
  async initIndexedDB() {
    if (!('indexedDB' in window)) {
      console.log('IndexedDB not available, using fallback method');
      this.indexedDBAvailable = false;
      return;
    }

    try {
      return new Promise((resolve, reject) => {
        const request = indexedDB.open(this.dbName, this.dbVersion);

        request.onerror = () => {
          console.warn('IndexedDB initialization failed, using fallback method');
          this.indexedDBAvailable = false;
          reject(new Error('IndexedDB initialization failed'));
        };

        request.onsuccess = () => {
          this.db = request.result;
          this.indexedDBAvailable = true;
          console.log('IndexedDB initialized successfully');
          resolve();
        };

        request.onupgradeneeded = (event) => {
          const db = event.target.result;
          // Recreate the store on every version bump: cached indexes are disposable
          // and may predate index format changes (e.g. the searchBoost field).
          if (db.objectStoreNames.contains(this.storeName)) {
            db.deleteObjectStore(this.storeName);
          }
          db.createObjectStore(this.storeName, { keyPath: 'cacheKey' });
        };
      });
    } catch (error) {
      console.warn('IndexedDB initialization error, using fallback method:', error);
      this.indexedDBAvailable = false;
      throw error;
    }
  }

  getCacheLanguage() {
    return this.currentLang === 'ru' ? 'ru' : 'en';
  }

  // Generate cache key from a single search index path using a hash function
  generateCacheKey(indexPath, lang = this.getCacheLanguage()) {
    // Use a hash function (djb2-like) to create a unique key from the path
    // This ensures different index paths and languages get different cache entries without collisions
    let hash = 5381; // djb2 initial value
    const str = `${indexPath}|${lang}`;
    for (let i = 0; i < str.length; i++) {
      hash = ((hash << 5) + hash) + str.charCodeAt(i);
      hash = hash | 0; // Convert to 32-bit integer
    }
    // Use absolute value and convert to hex for a clean, collision-resistant key
    const hashHex = Math.abs(hash).toString(16);
    return `searchIndex_${hashHex}`;
  }

  // Get cached search data for a single index file from IndexedDB
  async getCachedSearchData(cacheKey, cacheExpirationMs = null) {
    if (!this.indexedDBAvailable || !this.db) {
      return null;
    }

    // Use provided cache expiration or fall back to default (1 hour)
    const expirationMs = cacheExpirationMs !== null ? cacheExpirationMs : this.cacheExpirationMs;

    try {
      return new Promise(async (resolve, reject) => {
        const transaction = this.db.transaction([this.storeName], 'readonly');
        const store = transaction.objectStore(this.storeName);
        const request = store.get(cacheKey);

        request.onsuccess = async () => {
          const result = request.result;
          if (!result) {
            resolve(null);
            return;
          }

          // Check if cache is expired using the provided expiration time
          const now = Date.now();
          const cacheAge = now - result.timestamp;
          const cacheExpirationMinutes = Math.round(expirationMs / 60000);

          if (cacheAge > expirationMs) {
            console.log(`Cached search index expired for ${cacheKey} (age: ${Math.round(cacheAge / 60000)} minutes of ${cacheExpirationMinutes} minutes), will reload from network`);
            // Delete expired cache and wait for deletion to complete
            await this.deleteCachedSearchData(cacheKey);
            resolve(null);
            return;
          }

          console.log(`Using cached search index for ${cacheKey} (age: ${Math.round(cacheAge / 60000)} minutes of ${cacheExpirationMinutes} minutes)`);
          resolve(result.data);
        };

        request.onerror = () => {
          console.warn('Error reading from IndexedDB cache');
          resolve(null); // Fall back to network on error
        };
      });
    } catch (error) {
      console.warn('Error accessing IndexedDB cache:', error);
      return null;
    }
  }

  // Store search data for a single index file in IndexedDB
  async storeCachedSearchData(cacheKey, searchData) {
    if (!this.indexedDBAvailable || !this.db) {
      return;
    }

    try {
      return new Promise((resolve, reject) => {
        const transaction = this.db.transaction([this.storeName], 'readwrite');
        const store = transaction.objectStore(this.storeName);
        const cacheEntry = {
          cacheKey: cacheKey,
          data: searchData,
          timestamp: Date.now()
        };
        const request = store.put(cacheEntry);

        request.onsuccess = () => {
          console.log(`Search index cached successfully for ${cacheKey}`);
          resolve();
        };

        request.onerror = () => {
          console.warn('Error storing search index in cache');
          resolve(); // Don't fail the whole operation if caching fails
        };
      });
    } catch (error) {
      console.warn('Error storing in IndexedDB cache:', error);
      // Don't throw - caching failure shouldn't break the app
    }
  }

  // Delete cached search data
  async deleteCachedSearchData(cacheKey) {
    if (!this.indexedDBAvailable || !this.db) {
      return;
    }

    try {
      return new Promise((resolve) => {
        const transaction = this.db.transaction([this.storeName], 'readwrite');
        const store = transaction.objectStore(this.storeName);
        const request = store.delete(cacheKey);

        request.onsuccess = () => {
          resolve();
        };

        request.onerror = () => {
          resolve(); // Don't fail if deletion fails
        };
      });
    } catch (error) {
      console.warn('Error deleting from IndexedDB cache:', error);
    }
  }

  // Parse search index paths with boost levels and optional cache time
  // Format: "path:boost:cacheTime" or "path:boost" or "path"
  // cacheTime is in minutes, defaults to 60 (1 hour) if not specified
  parseSearchIndexPaths(searchIndexPath) {
    const paths = searchIndexPath.split(',').map(path => path.trim());

    return paths.map(path => {
      // Check if path contains boost level and cache time (format: "path:boost:cacheTime")
      const fullMatch = path.match(/^(.+):(\d+(?:\.\d+)?):(\d+)$/);
      if (fullMatch) {
        return {
          path: fullMatch[1].trim(),
          boost: parseFloat(fullMatch[2]),
          cacheTimeMinutes: parseInt(fullMatch[3], 10)
        };
      }

      // Check if path contains boost level only (format: "path:boost")
      const boostMatch = path.match(/^(.+):(\d+(?:\.\d+)?)$/);
      if (boostMatch) {
        return {
          path: boostMatch[1].trim(),
          boost: parseFloat(boostMatch[2]),
          cacheTimeMinutes: 60 // Default 1 hour
        };
      }

      // No boost or cache time specified - use defaults
      return {
        path: path,
        boost: 1.0,
        cacheTimeMinutes: 60 // Default 1 hour
      };
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
          // Re-running an unchanged query would call resetPagination() and collapse
          // blocks the user has already expanded (regaining focus after a "show more"
          // click used to do exactly that). Rendered results are still in the DOM.
          if (query !== this.lastQuery || !this.searchResults.querySelector('.result-item')) {
            this.handleSearch(query);
          }
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

    // A button inside the dropdown takes focus on mousedown, which blurs the input
    // and forces the click handler to restore focus - and that focus event used to
    // re-run the search. Suppressing the default keeps focus in the input all along.
    this.searchResults.addEventListener('mousedown', (e) => {
      if (e.target.closest('button')) {
        e.preventDefault();
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

        const showMoreButton = e.target.closest('.tile__pagination');
        if (showMoreButton) {
          this.loadMore(showMoreButton.dataset.groupType);
        }
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

    // An on-demand load is already running: joining it silently keeps its UI branch,
    // which reports progress in the dropdown the user is looking at.
    if (this.searchIndexLoadPromise) {
      await this.searchIndexLoadPromise;
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

  // Single-flight wrapper: focus starts a load without awaiting it, and the first
  // keystroke awaits one too. Without sharing the in-flight promise both ran, so the
  // worker got a second INIT and rebuilt the index - hence lunr's "Overwriting
  // existing registered function: lunr-multi-trimmer-en-ru".
  loadSearchIndex() {
    if (this.isDataLoaded) {
      return Promise.resolve();
    }
    if (!this.searchIndexLoadPromise) {
      this.searchIndexLoadPromise = this.loadSearchIndexOnce()
        .finally(() => {
          this.searchIndexLoadPromise = null;
        });
    }
    return this.searchIndexLoadPromise;
  }

  async loadSearchIndexOnce() {
    try {
      // Refresh language before reading cache to keep entries separated by page language.
      this.refreshLanguageDetection();
      const cacheLang = this.getCacheLanguage();

      // Only show loading UI if not loading in background
      if (!this.isLoadingInBackground) {
        this.showLoading();
      }

      // Parse search index paths with boost levels
      const indexConfigs = this.parseSearchIndexPaths(this.options.searchIndexPath);

      // Load each index file separately (from cache or network)
      const loadedIndexes = await Promise.all(
        indexConfigs.map(async (config) => {
          // Generate cache key for this specific index file
          const cacheKey = this.generateCacheKey(config.path, cacheLang);

          // Convert cache time from minutes to milliseconds
          const cacheExpirationMs = config.cacheTimeMinutes * 60 * 1000;

          // Try to load from cache first
          let indexData = await this.getCachedSearchData(cacheKey, cacheExpirationMs);

          if (indexData) {
            // Use cached data, but ensure boost is set
            indexData.boost = config.boost;
            return { data: indexData, config: config, fromCache: true };
          }

          // Cache miss or expired - load from network
          try {
            const response = await fetch(config.path);
            if (!response.ok) {
              console.warn(`Failed to load search index: ${config.path} (${response.status})`);
              return { data: { documents: [], parameters: [], boost: config.boost }, config: config, fromCache: false };
            }
            const data = await response.json();
            data.boost = config.boost;

            // Cache the loaded data for this specific index file
            await this.storeCachedSearchData(cacheKey, data);

            return { data: data, config: config, fromCache: false };
          } catch (error) {
            console.warn(`Error loading search index: ${config.path}`, error);
            return { data: { documents: [], parameters: [], boost: config.boost }, config: config, fromCache: false };
          }
        })
      );

      // Merge all search indexes with boost information
      this.searchData = {
        documents: [],
        parameters: [],
        indexBoosts: {} // Store boost levels for each index
      };

      loadedIndexes.forEach(({ data, config, fromCache }) => {
        const indexData = data;
        const indexPath = config.path;

        if (indexData && indexData.documents) {
          // Add boost information to each document
          const boostedDocuments = indexData.documents.map(doc => ({
            ...doc,
            _indexBoost: indexData.boost,
            _indexSource: indexPath
          }));
          this.searchData.documents = this.searchData.documents.concat(boostedDocuments);
          const source = fromCache ? 'cache' : 'network';
          console.log(`Added ${indexData.documents.length} documents from ${indexPath} (${source})`);
        }
        if (indexData && indexData.parameters) {
          // Add boost information to each parameter
          const boostedParameters = indexData.parameters.map(param => ({
            ...param,
            _indexBoost: indexData.boost,
            _indexSource: indexPath
          }));
          this.searchData.parameters = this.searchData.parameters.concat(boostedParameters);
          const source = fromCache ? 'cache' : 'network';
          console.log(`Added ${indexData.parameters.length} parameters from ${indexPath} (${source})`);
        }
        // Store boost level for this index
        this.searchData.indexBoosts[indexPath] = indexData.boost;
      });

      console.log(`Merged search data: ${this.searchData.documents.length} documents, ${this.searchData.parameters.length} parameters`);

      // Refresh language detection before building index
      this.refreshLanguageDetection();

      await this.initializeSearchEngine();
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
    // Inside lunr(function() {...}) `this` is the Lunr builder, so class methods have
    // to be captured beforehand - calling this.normalizeKeywords() there throws.
    const normalizeKeywords = (keywords) => this.normalizeKeywords(keywords);

    // Use multilingual support for Russian, default for English
    this.lunrIndex = lunr(function() {
      // Configure language support
      if (useRussianSupport) {
        this.use(lunr.multiLanguage('en', 'ru'));
      }

      // Configure fields
      this.field('title', { boost: 10 });
      this.field('keywords', { boost: 9 });
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
            keywords: normalizeKeywords(doc.keywords),
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
            keywords: normalizeKeywords(param.keywords),
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

  // Splits keywords into normalized items; string values are split by commas.
  parseKeywords(keywords) {
    if (Array.isArray(keywords)) {
      return keywords
        .filter((keyword) => typeof keyword === 'string')
        .flatMap((keyword) => keyword.split(','))
        .map((keyword) => keyword.trim())
        .filter((keyword) => keyword.length > 0);
    }
    if (typeof keywords === 'string') {
      return keywords
        .split(',')
        .map((keyword) => keyword.trim())
        .filter((keyword) => keyword.length > 0);
    }
    return [];
  }

  // Converts parsed keywords to search text used by Lunr/boosting.
  normalizeKeywords(keywords) {
    return this.parseKeywords(keywords).join(' ');
  }

  // Converts the page-level searchBoost front matter value to a multiplier.
  // The value is not capped: overriding a title match legitimately needs a large
  // multiplier. Anything missing, non-numeric or non-positive falls back to 1.
  normalizeSearchBoost(searchBoost) {
    const value = typeof searchBoost === 'string' ? parseFloat(searchBoost) : searchBoost;

    if (typeof value !== 'number' || !isFinite(value) || value <= 0) {
      return 1;
    }

    return value;
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
        const docKeywords = this.parseKeywords(doc.keywords);
        if (docKeywords.length > 0) {
          docKeywords.forEach((keyword) => {
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
        const paramKeywords = this.parseKeywords(param.keywords);
        if (paramKeywords.length > 0) {
          paramKeywords.forEach((keyword) => {
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

    // A query is plain text: everything Lunr would read as syntax is neutralized here.
    const sanitized = LUNR_SYNTAX_RULES
      .reduce((value, [pattern, replacement]) => value.replace(pattern, replacement), query)
      .replace(/\s+/g, ' ')
      .trim();

    return sanitized === query ? query : sanitized;
  }

  // A colon means the user pasted a "key: value" pair from a manifest, where both parts
  // have to appear on the page. Sanitization has already turned the colon into a space
  // by the time the query is searched, so the hint is read from the raw input.
  isKeyValueQuery(query) {
    return /[\p{L}\p{N}][ \t]*:[ \t]*[\p{L}\p{N}]/u.test(String(query == null ? '' : query));
  }

  normalizeSynonymKey(value) {
    return String(value || '')
      .toLowerCase()
      .replace(/\s+/g, ' ')
      .trim();
  }

  // Builds the directed lookup map (normalized term -> extra queries) out of
  // synonymGroups plus the optional one-way synonyms overrides. Keys are
  // normalized here, because lookups always go through normalizeSynonymKey().
  // Both sides are sanitized once, at build time: the map is looked up with an
  // already-sanitized query, and the worker receives it ready to search with.
  getNormalizedSynonyms() {
    if (this.normalizedSynonyms) {
      return this.normalizedSynonyms;
    }

    const normalized = {};
    const addLink = (from, to) => {
      const key = this.sanitizeQueryForSearch(this.normalizeSynonymKey(from));
      const value = this.sanitizeQueryForSearch(this.normalizeSynonymKey(to));
      if (!key || !value || key === value) {
        return;
      }
      if (!normalized[key]) {
        normalized[key] = [];
      }
      if (!normalized[key].includes(value)) {
        normalized[key].push(value);
      }
    };

    (this.options.synonymGroups || []).forEach((group) => {
      const members = (Array.isArray(group) ? group : [group])
        .filter((member) => typeof member === 'string' && member.trim().length > 0);
      members.forEach((member) => {
        members.forEach((other) => addLink(member, other));
      });
    });

    const directed = this.options.synonyms || {};
    Object.keys(directed).forEach((key) => {
      const values = Array.isArray(directed[key]) ? directed[key] : [directed[key]];
      values.forEach((value) => addLink(key, value));
    });

    this.normalizedSynonyms = normalized;
    return normalized;
  }

  // Looks up synonyms for the whole query and for every word window inside it,
  // so "провайдеры аутентификации в dex" still expands to dexprovider.
  getSynonymCandidates(query) {
    const normalizedQuery = this.normalizeSynonymKey(query);
    if (!normalizedQuery) {
      return [];
    }

    const synonymMap = this.getNormalizedSynonyms();
    const words = normalizedQuery.split(' ').filter(Boolean);
    const lookupKeys = new Set([normalizedQuery]);
    const maxWindow = Math.min(words.length, 4);
    for (let size = maxWindow; size >= 1; size--) {
      for (let start = 0; start + size <= words.length; start++) {
        lookupKeys.add(words.slice(start, start + size).join(' '));
      }
    }

    const candidates = [];
    const seen = new Set([normalizedQuery]);
    lookupKeys.forEach((key) => {
      const rawCandidates = synonymMap[key];
      if (!rawCandidates) {
        return;
      }
      const items = Array.isArray(rawCandidates) ? rawCandidates : [rawCandidates];
      items.forEach((item) => {
        const candidate = this.normalizeSynonymKey(item);
        if (!candidate || seen.has(candidate)) {
          return;
        }
        seen.add(candidate);
        candidates.push(candidate);
      });
    });

    return candidates;
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

    if (!this.useSearchWorker && !this.lunrIndex) {
      this.showError('Search index not loaded yet.');
      return;
    }

    try {
      // Sanitize the query to handle URLs and other problematic patterns
      const sanitizedQuery = this.sanitizeQueryForSearch(query);
      // Read from the raw input: sanitization has already turned the colon into a space.
      const requireAllWords = this.isKeyValueQuery(query);

      this.lastQuery = query; // Keep original query for display
      this.resetPagination();

      // A query built only from operators ("*", "+") sanitizes down to nothing, and an
      // empty Lunr query matches the entire corpus - 7154 pages on the current index.
      if (!sanitizedQuery.trim()) {
        this.showNoResults(query);
        return;
      }

      // Clear any existing fuzzy search messages
      this.clearFuzzySearchMessages();

      // Search is executed in worker when available, otherwise in main thread.
      let results = [];
      let highlightQuery = sanitizedQuery; // Use sanitized query for highlighting
      // Terms the result set was matched by: query itself plus applied synonyms
      // and fuzzy fallbacks, so all of them get highlighted in snippets.
      let highlightTerms = this.expandQueryHighlightTerms(sanitizedQuery);

      if (this.useSearchWorker && this.workerInitialized) {
        try {
          const workerResponse = await this.searchWithWorker(sanitizedQuery, requireAllWords);
          results = workerResponse.results || [];
          highlightQuery = workerResponse.highlightQuery || sanitizedQuery;
          highlightTerms = (workerResponse.highlightTerms && workerResponse.highlightTerms.length > 0)
            ? workerResponse.highlightTerms
            : this.expandQueryHighlightTerms(highlightQuery);
        } catch (workerError) {
          console.warn('Worker search failed, falling back to main thread:', workerError);
          this.useSearchWorker = false;
          if (!this.lunrIndex) {
            this.buildLunrIndex();
            this.buildSearchDictionary();
            this.buildFuseIndex();
            this.extractAvailableModules();
          }
        }
      }

      if (!this.useSearchWorker || !this.workerInitialized) {
        const mergeSearchResults = (baseResults, synonymResults, synonymBoost = 1.15) => {
          const mergedByRef = new Map();

          baseResults.forEach((result) => {
            mergedByRef.set(result.ref, result);
          });

          synonymResults.forEach((result) => {
            const boostedSynonymResult = {
              ...result,
              score: (result.score || 0) * synonymBoost
            };
            const existing = mergedByRef.get(result.ref);
            if (!existing || (existing.score || 0) < boostedSynonymResult.score) {
              mergedByRef.set(result.ref, boostedSynonymResult);
            }
          });

          return Array.from(mergedByRef.values());
        };

        const searchWithFallback = (inputQuery) => {
          try {
            return {
              results: this.lunrIndex.search(inputQuery),
              highlightQuery: inputQuery
            };
          } catch (error) {
            const fallbackQuery = inputQuery.replace(/[^\w\s-]/g, ' ').replace(/\s+/g, ' ').trim();
            if (fallbackQuery !== inputQuery) {
              return {
                results: this.lunrIndex.search(fallbackQuery),
                highlightQuery: fallbackQuery
              };
            }
            throw error;
          }
        };

        // "kind: configmap" is a key/value pair, so both words are required: that is 29
        // pages against 381 when the same words are OR-ed. Pages that never mention them
        // together are still reachable, because an empty strict result falls back to OR.
        const searchKeyValueAware = (plainQuery) => {
          if (requireAllWords) {
            const strictQuery = this.buildPhraseQuery(plainQuery);
            if (strictQuery !== plainQuery) {
              const strictSearch = searchWithFallback(strictQuery);
              if (strictSearch.results.length > 0) {
                // Highlighting keeps the plain query: "+word" is a Lunr operator, not text.
                return { results: strictSearch.results, highlightQuery: plainQuery };
              }
            }
          }
          return searchWithFallback(plainQuery);
        };

        try {
          const initialSearch = searchKeyValueAware(sanitizedQuery);
          results = initialSearch.results;
          highlightQuery = initialSearch.highlightQuery;
          highlightTerms = this.expandQueryHighlightTerms(highlightQuery);
        } catch (error) {
          console.warn('Lunr search error with sanitized query:', error);
          this.showError('Search query contains invalid characters. Please try a different search term.');
          return;
        }

        // Try mapped synonyms and merge their matches with the original result set.
        const synonymCandidates = this.getSynonymCandidates(sanitizedQuery);
        const synonymResults = [];
        for (const synonymQuery of synonymCandidates) {
          try {
            const synonymSearch = searchWithFallback(this.buildPhraseQuery(synonymQuery));
            if (synonymSearch.results.length > 0) {
              synonymResults.push(...synonymSearch.results);
              // Highlighted as written: matched as a whole phrase, never word by word.
              highlightTerms.push(synonymQuery);
            }
          } catch (synonymError) {
            console.warn('Synonym search failed:', synonymError);
          }
        }
        if (synonymResults.length > 0) {
          const hadInitialResults = results.length > 0;
          results = mergeSearchResults(results, synonymResults);
          if (!hadInitialResults) {
            highlightQuery = sanitizedQuery;
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
            highlightTerms.push(...this.expandQueryHighlightTerms(bestSuggestion));
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
              highlightTerms.push(...this.expandQueryHighlightTerms(suggestion.item));
              break;
            }
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

        // Apply per-document boost declared in page front matter
        boost *= this.normalizeSearchBoost(doc.searchBoost);

        // Check if the search query matches the module name
        const queryLower = sanitizedQuery.toLowerCase();
        const moduleLower = (doc.module || '').toLowerCase();

        if (moduleLower && moduleLower.includes(queryLower)) {
          boost *= 1.8; // Strong boost for module name matches
        }

        // Check for parameter field matches with specific priority order
        if (doc.type === 'parameter') {
          const nameLower = (doc.name || '').toLowerCase();
          const keywordsLower = this.normalizeKeywords(doc.keywords).toLowerCase();
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
            boost *= 3; // Moderate-high boost for keyword matches
          }

          // Priority 3: Content field matches (lowest priority for parameters)
          if (contentLower && contentLower.includes(queryLower)) {
            boost *= 1.2; // Low boost for content matches
          }
        } else {
          // For non-parameters (documents), use document field priority order
          const titleLower = (doc.title || '').toLowerCase();
          const keywordsLower = this.normalizeKeywords(doc.keywords).toLowerCase();
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
            boost *= 3; // Moderate-high boost for keyword matches
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
      this.currentHighlightTerms = Array.from(new Set(highlightTerms.filter(Boolean)));
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

    // Highlight by every matched term (query + synonyms), not just the raw query.
    const highlightTerms = (this.currentHighlightTerms && this.currentHighlightTerms.length > 0)
      ? this.currentHighlightTerms
      : [this.currentHighlightQuery || this.lastQuery];

    // Display Modules as a row at the top
    if (this.currentResults.modules.length > 0) {
      resultsHtml += this.renderModulesRow(this.currentResults.modules, highlightTerms);
    }

    // The four API subgroups are a single list for the user: they share one header,
    // so they are concatenated in priority order and paginated by one counter.
    // Rendering them separately would put up to four "show more" buttons in the
    // block, each extending its own subgroup somewhere in the middle of the list.
    const apiResults = [
      ...this.currentResults.isResourceNameMatch,
      ...this.currentResults.nameMatch,
      ...this.currentResults.isResourceOther,
      ...this.currentResults.parameterOther
    ];

    if (apiResults.length > 0) {
      resultsHtml += `
        <div class="results-group">
          <div class="results-group-header">${this.t('api')}</div>
          ${this.renderResultGroup(apiResults, highlightTerms, 'api')}
        </div>
      `;
    }

    // Display documentation results (only from documents array)
    if (this.currentResults.document.length > 0) {
      resultsHtml += `
        <div class="results-group">
          <div class="results-group-header">${this.t('documentation')}</div>
          ${this.renderResultGroup(this.currentResults.document, highlightTerms, 'document')}
        </div>
      `;
    }

    this.searchResults.innerHTML = resultsHtml;
  }

  renderModulesRow(results, highlightTerms) {
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

  renderBreadcrumbsRow(breadcrumbs, highlightTerms) {
    if (!Array.isArray(breadcrumbs) || breadcrumbs.length === 0) {
      return '';
    }

    const maxPathLength = 100;
    const normalizedItems = breadcrumbs
      .filter((item) => typeof item === 'string' && item.trim().length > 0)
      .map((item) => item.trim());

    if (normalizedItems.length === 0) {
      return '';
    }

    const visibleItems = [];
    let currentLength = 0;
    let isTruncated = false;

    for (let i = 0; i < normalizedItems.length; i++) {
      const item = normalizedItems[i];
      const separatorLength = i > 0 ? 3 : 0; // " > "
      const nextLength = currentLength + separatorLength + item.length;
      if (nextLength > maxPathLength) {
        isTruncated = true;
        break;
      }
      visibleItems.push(item);
      currentLength = nextLength;
    }

    const breadcrumbBadges = visibleItems.map((item, index) => {
      const separator = index > 0 ? '<span class="result-breadcrumbs-separator">→</span>' : '';
      return `${separator}<span class="result-breadcrumbs">${this.highlightText(item, highlightTerms)}</span>`;
    });

    if (isTruncated) {
      breadcrumbBadges.push('<span class="result-breadcrumbs-separator">→</span><span class="result-breadcrumbs-ellipsis">...</span>');
    }

    if (breadcrumbBadges.length === 0) {
      return '';
    }

    return `<div class="breadcrumbs-row">${breadcrumbBadges.join('')}</div>`;
  }

  renderResultGroup(results, highlightTerms, groupType) {
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

      let title, module, description, breadcrumbs;

      // Markup is chosen per result, not per group: the API block mixes subgroups,
      // and doc.type is set for every grouped result by groupResults().
      const isParameter = doc.type === 'parameter' || (!doc.title && !!doc.name);

      if (isParameter) {
        // For configuration results (parameters) and isResource parameters
        title = this.highlightText(doc.name || '', highlightTerms);
        module = doc.module ? `<div class="result-module">${doc.module}</div>` : '';
        if (doc.resName != doc.name) {
          module += doc.resName ? `<div class="result-module">${doc.resName}</div>` : '';
        }
        breadcrumbs = this.renderBreadcrumbsRow(doc.bc, highlightTerms);
        description = this.highlightText(this.getRelevantContentSnippet(doc.content || '', highlightTerms), highlightTerms);
      } else {
        // For other documentation
        title = this.highlightText(doc.title || '', highlightTerms);
        module = doc.module ? `<div class="result-module">${doc.module}</div>` : '';
        breadcrumbs = this.renderBreadcrumbsRow(doc.bc, highlightTerms);
        description = this.highlightText(this.getRelevantContentSnippet(doc.content || '', highlightTerms), highlightTerms);
      }

      html += `
        <a href="${this.buildTargetUrl(doc.url, doc.moduletype, doc.module)}" class="result-item">
          <div class="result-title">${title}</div>
          ${module}
          ${breadcrumbs}
          <div class="result-description">${description}</div>
        </a>
      `;
    });

    // Without this button the group is capped at its initial count with no way
    // to reach the rest of the matches. Clicks are handled by delegation in
    // setupEventListeners(), which already keeps the dropdown open.
    if (displayedCount < results.length) {
      const remaining = Math.min(this.pageSize, results.length - displayedCount);
      html += `
        <button type="button" class="tile__pagination" data-group-type="${groupType}">
          <p class="tile__pagination--descr">${this.t('showMorePattern', { count: remaining })}</p>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path fill-rule="evenodd" clip-rule="evenodd" d="M8 1C8.55229 1 9 1.44772 9 2V7L14 7C14.5523 7 15 7.44772 15 8C15 8.55229 14.5523 9 14 9L9 9L9 14C9 14.5523 8.55229 15 8 15C7.44772 15 7 14.5523 7 14L7 9H2C1.44772 9 1 8.55229 1 8C1 7.44772 1.44772 7 2 7L7 7L7 2C7 1.44772 7.44772 1 8 1Z" fill="#0D69F2"/>
          </svg>
        </button>
      `;
    }

    return html;
  }

  loadMore(groupType) {
    if (!Object.prototype.hasOwnProperty.call(this.displayedCounts, groupType)) {
      return;
    }

    // displayResults() replaces innerHTML, which resets the dropdown scroll:
    // keep the position so the list grows below the button instead of jumping up.
    const scrollTop = this.searchResults ? this.searchResults.scrollTop : 0;
    this.displayedCounts[groupType] += this.pageSize;
    this.displayResults();
    if (this.searchResults) {
      this.searchResults.scrollTop = scrollTop;
    }
  }

  resetPagination() {
    this.displayedCounts = {
      api: this.pageSize,
      document: this.pageSize
    };
  }

  // Returns a plain-text snippet; callers highlight it once via highlightText().
  // Every highlight term (query, its words, synonyms) participates in scoring, so a
  // snippet mentioning only the synonym (dexprovider) still wins for a RU query.
  getRelevantContentSnippet(content, terms) {
    if (!content) return '';

    const highlightTerms = this.normalizeHighlightTerms(terms);

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
    if (sentences.length === 0) return '';

    const scoredTerms = highlightTerms
      .map((term) => ({ term, regex: this.buildHighlightRegex([term]) }))
      .filter((entry) => entry.regex);

    let best = null;
    sentences.forEach((sentence) => {
      // Score against tag-free text: a hit inside markup would never be highlighted.
      const plainSentence = sentence.replace(/<[^>]*>/g, ' ');
      let matchedTerms = 0;
      let score = 0;

      scoredTerms.forEach(({ term, regex }) => {
        regex.lastIndex = 0;
        if (regex.test(plainSentence)) {
          matchedTerms++;
          score += term.length; // Longer terms and whole phrases weigh more
        }
      });

      if (matchedTerms === 0) return;

      // Prefer sentences covering more distinct terms, then the heavier match.
      const weight = matchedTerms * 1000 + score;
      if (!best || weight > best.weight) {
        best = { sentence, weight };
      }
    });

    // Fallback: take the first sentence and truncate
    const snippet = (best ? best.sentence : sentences[0]).trim();
    return snippet.length > 200 ? truncateText(snippet, 200) : snippet;
  }

  escapeRegExp(value) {
    return String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  // Flattens highlight input (string or array) into a unique term list: whole
  // phrases first, then their separate words, longest first so phrases win.
  normalizeHighlightTerms(terms) {
    const source = Array.isArray(terms) ? terms : [terms];
    const collected = [];
    const seen = new Set();

    const push = (value) => {
      const term = String(value == null ? '' : value).trim();
      if (term.length < 2) return;
      const key = term.toLowerCase();
      if (seen.has(key)) return;
      seen.add(key);
      collected.push(term);
    };

    // Terms arrive ready to use: query words are split by expandQueryHighlightTerms(),
    // while a synonym phrase stays whole so "siem" does not mark every "management".
    source.forEach(push);

    return collected.sort((a, b) => b.length - a.length);
  }

  // A query is an OR over its words, and a document may match just one of them, so
  // every word is highlighted on its own alongside the full string. Synonym phrases
  // are excluded from this: they are matched whole, see buildPhraseQuery().
  expandQueryHighlightTerms(query) {
    const value = String(query == null ? '' : query).trim();
    if (!value) {
      return [];
    }
    return /\s/.test(value) ? [value, ...value.split(/\s+/)] : [value];
  }

  // Lunr has no phrase queries: a multi-word synonym passed as a plain string is an OR
  // over its words, so "siem" would drag in every page that merely says "management"
  // (1031 hits against 3 for the phrase as a whole). Requiring every word instead
  // ("+security +information +event +management") keeps the match tied to the phrase.
  buildPhraseQuery(phrase) {
    const words = String(phrase == null ? '' : phrase).trim().split(/\s+/).filter(Boolean);
    if (words.length < 2) {
      return phrase;
    }

    // Bail out instead of risking a parse error on Lunr query operators (+ - : ^ ~ *).
    if (words.some((word) => !/^[\p{L}\p{N}_./-]+$/u.test(word))) {
      return phrase;
    }

    const required = words.filter((word) => !this.isSearchStopWord(word));

    if (required.length === 0) {
      return phrase;
    }
    if (required.length === 1) {
      return required[0];
    }

    return required.map((word) => `+${word}`).join(' ');
  }

  // Stop words are dropped when the index is built, so a required clause made of one
  // ("+and") empties the entire result set. The search pipeline keeps them, unlike the
  // indexing pipeline, so they have to be recognized explicitly.
  isSearchStopWord(word) {
    const lower = String(word == null ? '' : word).toLowerCase();
    const filters = typeof lunr === 'undefined'
      ? []
      : [lunr.stopWordFilter, lunr.ru ? lunr.ru.stopWordFilter : null];

    return filters.some((filter) => {
      if (typeof filter !== 'function') return false;
      try {
        return !filter(lower);
      } catch (error) {
        return false;
      }
    });
  }

  // Builds a regexp source for a single term, tolerating inflections so the
  // query "провайдеры" also marks "провайдеров" in the text.
  buildHighlightTermPattern(term) {
    if (/\s/.test(term)) {
      // Phrase: allow any whitespace run between words.
      return term.split(/\s+/).map((word) => this.buildHighlightTermPattern(word)).join('\\s+');
    }

    const escaped = this.escapeRegExp(term);
    if (term.length < 5 || !/^[\p{L}\p{N}]+$/u.test(term)) {
      return escaped;
    }

    const isCyrillic = /\p{Script=Cyrillic}/u.test(term);
    if (!isCyrillic) {
      // Latin: allow a short suffix (plural, -ing, -ed).
      return `${escaped}[\\p{L}]{0,2}`;
    }

    const cut = term.length >= 8 ? 3 : 2;
    return `${this.escapeRegExp(term.slice(0, term.length - cut))}[\\p{L}]{0,4}`;
  }

  // Compiles (and caches) one alternation regex covering all highlight terms.
  buildHighlightRegex(terms) {
    const normalized = this.normalizeHighlightTerms(terms);
    if (normalized.length === 0) return null;

    if (!this.highlightRegexCache) {
      this.highlightRegexCache = new Map();
    }

    const cacheKey = normalized.join('\u0000');
    if (this.highlightRegexCache.has(cacheKey)) {
      return this.highlightRegexCache.get(cacheKey);
    }

    const pattern = `(${normalized.map((term) => this.buildHighlightTermPattern(term)).join('|')})`;
    let regex = null;
    try {
      // Lookbehind keeps matches at word starts, so a stem never lights up the
      // middle of an unrelated word.
      regex = new RegExp(`(?<![\\p{L}\\p{N}])${pattern}`, 'giu');
    } catch (error) {
      try {
        regex = new RegExp(pattern, 'giu');
      } catch (fallbackError) {
        console.warn('Failed to build highlight regex:', fallbackError);
        regex = null;
      }
    }

    if (this.highlightRegexCache.size > 64) {
      this.highlightRegexCache.clear();
    }
    this.highlightRegexCache.set(cacheKey, regex);
    return regex;
  }

  highlightText(text, terms) {
    if (!text) return '';

    const regex = this.buildHighlightRegex(terms);
    if (!regex) return text;

    // Snippets may carry markup, so only text between tags is marked up: otherwise
    // a match inside an attribute (href="/dexprovider/") would corrupt the HTML.
    return String(text)
      .split(/(<[^>]*>)/)
      .map((chunk) => (chunk.charAt(0) === '<' ? chunk : chunk.replace(regex, '<mark>$1</mark>')))
      .join('');
  }

  buildTargetUrl(originalTargetUrl, moduleType = null, moduleName = null) {
    // console.debug('buildTargetUrl called with:', originalTargetUrl, 'moduleType:', moduleType, 'moduleName:', moduleName);
    const dkpDocBaseUrl = "/products/kubernetes-platform/documentation/v1/";


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
      if (isCurrentModulePage && currentPageUrlWithoutVersion.endsWith(relativeCurrentPageURL) && isModuleResult) {
        baseUrl = currentPageUrlWithoutVersion.substring(0, currentPageUrlWithoutVersion.length - relativeCurrentPageURL.length);
        // console.debug('Base URL calculated:', baseUrl);
      } if (isCurrentModulePage && !isModuleResult) {
        baseUrl = dkpDocBaseUrl;
        console.debug('Base URL calculated (from module to DKP doc):', baseUrl);
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

    if (isCurrentModulePage && !originalTargetUrl.startsWith('/modules/')) {
      console.debug('No meta tag found and link from module to DKP doc, returning:', dkpDocBaseUrl + originalTargetUrl);
      return dkpDocBaseUrl + originalTargetUrl;
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

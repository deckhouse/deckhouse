/*
 * Dedicated search worker protocol:
 * - Main -> Worker:
 *   1) { type: 'INIT', payload: { searchData, currentLang } }
 *      Builds Lunr/Fuse indexes and module list in worker thread.
 *   2) { type: 'SEARCH', payload: { requestId, query } }
 *      Runs search for user query.
 *
 * - Worker -> Main:
 *   1) { type: 'READY', payload: { availableModules } }
 *      Sent when initialization is complete and worker can accept SEARCH.
 *   2) { type: 'SEARCH_RESULT', payload: { requestId, results, highlightQuery } }
 *      Search result for a specific requestId.
 *   3) { type: 'ERROR', payload: { message } }
 *      Initialization/runtime error.
 *      For search-time errors: { type: 'ERROR', payload: { requestId, message } }.
 */

let searchData = {
  documents: [],
  parameters: []
};
let currentLang = 'en';
let lunrIndex = null;
let searchDictionary = [];
let fuseIndex = null;
let availableModules = [];
let synonyms = {
  'update policy': ['moduleupdatepolicy'],
  'dex Providers': ['dexprovider'],
  'провайдеры аутентификации': ['dexprovider'],
  'переопределение': ['modulepulloverride'],
  moduleupdatepolicy: ['update policy', 'module update policy', 'политика обновления'],
  dexprovider: ['провайдеры аутентификации', 'dex providers'],
  modulepulloverride: ['переопределение']
};

function parseKeywords(keywords) {
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

function normalizeKeywords(keywords) {
  return parseKeywords(keywords).join(' ');
}

try {
  self.importScripts(
    '/assets/js/lunr/lunr.js',
    '/assets/js/lunr/lunr.stemmer.support.js',
    '/assets/js/lunr/lunr.multi.js',
    '/assets/js/lunr/lunr.ru.js',
    '/assets/js/fuse.min.js'
  );
} catch (error) {
  self.postMessage({
    type: 'ERROR',
    payload: { message: `Failed to load search worker dependencies: ${error.message}` }
  });
}

// Builds Lunr index from documents and parameters inside worker thread.
function buildLunrIndex() {
  const useRussianSupport = currentLang === 'ru' && typeof lunr.multiLanguage !== 'undefined';

  lunrIndex = lunr(function () {
    if (useRussianSupport) {
      this.use(lunr.multiLanguage('en', 'ru'));
    }

    this.field('title', { boost: 10 });
    this.field('keywords', { boost: 8 });
    this.field('module', { boost: 6 });
    this.field('summary', { boost: 3 });
    this.field('content', { boost: 1 });
    this.ref('id');

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

        if (useRussianSupport && doc.moduletype) {
          docData.moduletype = doc.moduletype;
        }

        this.add(docData);
        docCounter++;
      });
    }

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

        if (useRussianSupport && param.moduletype) {
          paramData.moduletype = param.moduletype;
        }

        this.add(paramData);
        paramCounter++;
      });
    }
  });
}

// Normalizes text and extracts searchable words for dictionary/fuzzy search.
function extractWords(text) {
  if (!text) return [];

  return text
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s-]/gu, ' ')
    .replace(/[-_]/g, ' ')
    .split(/\s+/)
    .filter(word => word.length >= 2)
    .filter(word => !/^\d+$/.test(word))
    .filter(word => /[\p{L}]/u.test(word));
}

// Collects unique terms used by Fuse fuzzy suggestions.
function buildSearchDictionary() {
  const dictionary = new Set();

  if (searchData.documents) {
    searchData.documents.forEach(doc => {
      if (doc.title) {
        extractWords(doc.title).forEach(word => dictionary.add(word));
      }
      const docKeywords = parseKeywords(doc.keywords);
      if (docKeywords.length > 0) {
        docKeywords.forEach((keyword) => {
          extractWords(keyword).forEach(word => dictionary.add(word));
        });
      }
      if (doc.module) {
        extractWords(doc.module).forEach(word => dictionary.add(word));
      }
      if (doc.summary) {
        extractWords(doc.summary).forEach(word => dictionary.add(word));
      }
    });
  }

  if (searchData.parameters) {
    searchData.parameters.forEach(param => {
      if (param.name) {
        extractWords(param.name).forEach(word => dictionary.add(word));
      }
      const paramKeywords = parseKeywords(param.keywords);
      if (paramKeywords.length > 0) {
        paramKeywords.forEach((keyword) => {
          extractWords(keyword).forEach(word => dictionary.add(word));
        });
      }
      if (param.module) {
        extractWords(param.module).forEach(word => dictionary.add(word));
      }
      if (param.resName) {
        extractWords(param.resName).forEach(word => dictionary.add(word));
      }
    });
  }

  searchDictionary = Array.from(dictionary)
    .filter(word => word.length >= 2)
    .sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
}

// Builds Fuse index used for typo-tolerant fallback search.
function buildFuseIndex() {
  if (typeof Fuse === 'undefined') {
    fuseIndex = null;
    return;
  }

  fuseIndex = new Fuse(searchDictionary, {
    threshold: 0.4,
    distance: 100,
    includeScore: true,
    minMatchCharLength: 2,
    ignoreLocation: true,
    findAllMatches: false,
    useExtendedSearch: false
  });
}

// Computes simple string similarity for Russian fallback matching.
function calculateRussianSimilarity(str1, str2) {
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
          matrix[i - 1][j - 1] + 1,
          matrix[i][j - 1] + 1,
          matrix[i - 1][j] + 1
        );
      }
    }
  }

  const distance = matrix[len2][len1];
  const maxLength = Math.max(len1, len2);
  return 1 - (distance / maxLength);
}

// Returns best fuzzy candidates for Cyrillic queries.
function getRussianFuzzySuggestions(query) {
  const queryLower = query.toLowerCase();
  const results = [];
  const russianTerms = searchDictionary.filter(term => /[а-яё]/i.test(term));

  for (const term of russianTerms) {
    const termLower = term.toLowerCase();
    let score = 0;

    if (termLower === queryLower) {
      score = 1.0;
    } else if (termLower.includes(queryLower)) {
      score = 0.8;
    } else if (queryLower.includes(termLower)) {
      score = 0.7;
    } else {
      const similarity = calculateRussianSimilarity(queryLower, termLower);
      if (similarity > 0.2) {
        score = similarity;
      }
    }

    if (score > 0.2) {
      results.push({ item: term, score: score });
    }
  }

  return results.sort((a, b) => b.score - a.score);
}

// Returns top fuzzy suggestions from Fuse (or RU fallback).
function getFuzzySuggestions(query) {
  if (!fuseIndex || !query.trim()) {
    return [];
  }

  let fuzzyResults = fuseIndex.search(query);
  const hasRussian = /[а-яё]/i.test(query);
  if (hasRussian && fuzzyResults.length === 0) {
    fuzzyResults = getRussianFuzzySuggestions(query);
  }

  return fuzzyResults.slice(0, 5);
}

// Sanitizes user query to avoid Lunr syntax/operator parse errors.
function sanitizeQueryForSearch(query) {
  const urlPattern = /^https?:\/\/[^\s]+$/i;
  if (urlPattern.test(query)) {
    try {
      const url = new URL(query);
      const domain = url.hostname.replace(/^www\./, '');
      const pathSegments = url.pathname.split('/').filter(segment => segment.length > 0);
      return [domain, ...pathSegments].join(' ');
    } catch (e) {
      return query.replace(/^https?:\/\//, '').replace(/[^\w\s-]/g, ' ').trim();
    }
  }

  let sanitized = query;
  let hasChanges = false;

  if (/^[a-zA-Z]*:/.test(sanitized)) {
    sanitized = sanitized.replace(/:/g, ' ');
    hasChanges = true;
  }

  if (sanitized.includes('--')) {
    sanitized = sanitized.replace(/--/g, ' ');
    hasChanges = true;
  }

  const lunrOperatorPattern = /(\s|^)[+\-](\w+)/g;
  if (lunrOperatorPattern.test(sanitized)) {
    sanitized = sanitized.replace(lunrOperatorPattern, '$1$2');
    hasChanges = true;
  }

  return hasChanges ? sanitized.trim() : query;
}

function normalizeSynonymKey(value) {
  return String(value || '')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim();
}

function getSynonymCandidates(query) {
  const normalizedQuery = normalizeSynonymKey(query);
  if (!normalizedQuery || !synonyms) {
    return [];
  }

  const rawCandidates = synonyms[normalizedQuery];
  if (!rawCandidates) {
    return [];
  }

  const items = Array.isArray(rawCandidates) ? rawCandidates : [rawCandidates];
  return items
    .map((item) => sanitizeQueryForSearch(normalizeSynonymKey(item)))
    .filter((item) => item && item !== normalizedQuery);
}

// Extracts unique module names for synthetic "module page" results in UI.
function buildAvailableModules() {
  const modules = new Set();

  if (searchData.documents) {
    searchData.documents.forEach(doc => {
      if (doc.module && doc.module.trim()) {
        modules.add(doc.module.trim());
      }
    });
  }

  if (searchData.parameters) {
    searchData.parameters.forEach(param => {
      if (param.module && param.module.trim()) {
        modules.add(param.module.trim());
      }
    });
  }

  availableModules = Array.from(modules);
}

// Executes Lunr search and applies fuzzy fallback strategy.
function runSearch(query) {
  const sanitizedQuery = sanitizeQueryForSearch(query);
  let results = [];
  let highlightQuery = sanitizedQuery;
  const searchWithFallback = (inputQuery) => {
    try {
      return {
        results: lunrIndex.search(inputQuery),
        highlightQuery: inputQuery
      };
    } catch (error) {
      const fallbackQuery = inputQuery.replace(/[^\w\s-]/g, ' ').replace(/\s+/g, ' ').trim();
      if (fallbackQuery !== inputQuery) {
        return {
          results: lunrIndex.search(fallbackQuery),
          highlightQuery: fallbackQuery
        };
      }
      throw error;
    }
  };

  const initialSearch = searchWithFallback(sanitizedQuery);
  results = initialSearch.results;
  highlightQuery = initialSearch.highlightQuery;

  if (results.length === 0) {
    const synonymCandidates = getSynonymCandidates(sanitizedQuery);
    for (const synonymQuery of synonymCandidates) {
      try {
        const synonymSearch = searchWithFallback(synonymQuery);
        if (synonymSearch.results.length > 0) {
          results = synonymSearch.results;
          highlightQuery = synonymSearch.highlightQuery;
          break;
        }
      } catch (synonymError) {
        // Ignore invalid synonym query and continue with next candidate.
      }
    }
  }

  if (results.length === 0 && fuseIndex) {
    const fuzzySuggestions = getFuzzySuggestions(sanitizedQuery);
    if (fuzzySuggestions.length > 0) {
      const bestSuggestion = fuzzySuggestions[0].item;
      results = lunrIndex.search(bestSuggestion);
      highlightQuery = bestSuggestion;
    }
  }

  if (results.length === 0 && fuseIndex) {
    const fuzzySuggestions = getFuzzySuggestions(sanitizedQuery);
    for (const suggestion of fuzzySuggestions.slice(0, 3)) {
      const wordResults = lunrIndex.search(suggestion.item);
      if (wordResults.length > 0) {
        results = wordResults;
        highlightQuery = suggestion.item;
        break;
      }
    }
  }

  return {
    results,
    highlightQuery
  };
}

// Handles worker protocol: INIT builds indexes, SEARCH returns matches.
self.onmessage = (event) => {
  const { type, payload } = event.data || {};

  if (type === 'INIT') {
    try {
      searchData = payload.searchData || { documents: [], parameters: [] };
      currentLang = payload.currentLang || 'en';
      synonyms = payload.synonyms || synonyms;
      buildLunrIndex();
      buildSearchDictionary();
      buildFuseIndex();
      buildAvailableModules();
      self.postMessage({
        type: 'READY',
        payload: {
          availableModules
        }
      });
    } catch (error) {
      self.postMessage({
        type: 'ERROR',
        payload: {
          message: `Failed to initialize search worker: ${error.message}`
        }
      });
    }
    return;
  }

  if (type === 'SEARCH') {
    const requestId = payload.requestId;
    try {
      if (!lunrIndex) {
        throw new Error('Search index is not initialized');
      }
      const query = payload.query || '';
      const result = runSearch(query);
      self.postMessage({
        type: 'SEARCH_RESULT',
        payload: {
          requestId,
          results: result.results,
          highlightQuery: result.highlightQuery
        }
      });
    } catch (error) {
      self.postMessage({
        type: 'ERROR',
        payload: {
          requestId,
          message: `Worker search failed: ${error.message}`
        }
      });
    }
  }
};

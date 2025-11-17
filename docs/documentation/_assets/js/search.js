class ModuleSearch {
  constructor() {
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

    this.init();
  }

  async init() {
    this.setupEventListeners();

    // Hide search results by default
    this.searchResults.style.display = 'none';
  }

  setupEventListeners() {
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

      // Close search results when clicking outside, but not if there are loading/error messages
      const hasLoadingOrError = this.searchResults.querySelector('.loading > .spinner-small, .no-results');
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
      }
    });
  }

  async loadSearchIndex() {
    if (this.isDataLoaded) {
      return; // Already loaded
    }

    try {
      this.showLoading();

      const response = await fetch('/modules/search-embedded-modules-index.json');
      if (!response.ok) {
        throw new Error(`Failed to load search index: ${response.status}`);
      }

      this.searchData = await response.json();
      this.buildLunrIndex();
      this.isDataLoaded = true;
      this.hideLoading();
    } catch (error) {
      console.error('Error loading search index:', error);
      this.showError('Failed to load search index. Please try again later.');
    }
  }

  buildLunrIndex() {
    const searchData = this.searchData;

    this.lunrIndex = lunr(function() {
      this.use(lunr.multiLanguage('en', 'ru'))
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
          <div class="results-group-header">{{< translate "api" >}}</div>
          ${this.renderResultGroup(this.currentResults.config, this.lastQuery, 'config')}
        </div>
      `;
    }

    // Display other results
    if (this.currentResults.other.length > 0) {
      resultsHtml += `
        <div class="results-group">
          <div class="results-group-header">{{< translate "Documentation" >}}</div>
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
        summary = this.highlightText(doc.resName || '', query);
        module = doc.module ? `<div class="result-module">${doc.module}</div>` : '';
        description = this.highlightText(doc.content || '', query);
      } else {
        // For other documentation
        title = this.highlightText(doc.title || '', query);
        summary = this.highlightText(doc.summary || '', query);
        module = doc.module ? `<div class="result-module">${doc.module}</div>` : '';
        description = summary || this.getRelevantContentSnippet(doc.content || '', query);
      }

      html += `
        <a href="${doc.url || '#'}" class="result-item">
          <div class="result-title">${title}</div>
          ${module}
          <div class="result-description">${description}</div>
        </a>
      `;
    });

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
        snippet = snippet.substring(0, 200) + '...';
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
        snippet = snippet.substring(0, 200) + '...';
      }
      return this.highlightText(snippet, query);
    }

    // Fallback: take the first sentence and truncate
    if (sentences.length > 0) {
      let snippet = sentences[0].trim();
      if (snippet.length > 200) {
        snippet = snippet.substring(0, 200) + '...';
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

  showLoading() {
    this.searchResults.style.display = 'flex';
    this.searchResults.innerHTML = '<div class="search-loading">Loading search index...</div>';
  }

  hideLoading() {
    // Loading will be replaced by results or message
  }

  showMessage(message) {
    this.searchResults.style.display = 'flex';
    this.searchResults.innerHTML = `<div class="no-results">${message}</div>`;
  }

  showNoResults(query) {
    this.searchResults.style.display = 'flex';
    this.searchResults.innerHTML = `
      <div class="no-results">
        No modules found for "${query}". Try different keywords or check your spelling.
      </div>
    `;
  }

  showError(message) {
    this.searchResults.style.display = 'flex';
    this.searchResults.innerHTML = `<div class="no-results">${message}</div>`;
  }
}

// Initialize search when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
  window.moduleSearch = new ModuleSearch();
});

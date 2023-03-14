<script>
export default {
  inject: ['$eventBus'],

  props: {
    filter: Object,
    loaderId: String
  },

  data() {
    return {
      // REQUIRES itemClass
      items: [],
      firstDisplayedRow: 0,
      totalRows: 0,
      pageCount: 0,
      itemsPerPage: 15,
      loading: true,
      displayQlParsingError: false // Flag for special case of non-critical error - query language syntax error.
    };
  },

  created() {
    // Even a static board has to react to local updates
    var vm = this;
    this.updateCB = this.itemClass.addChannelCallback('update', function(item, oldVal) {
      var idx = vm.items.indexOf(item);
      if (idx > -1 && item.status != vm.filter.status) {
        vm.items.splice(idx, 1);
      }
    });

    this.loadAuxData().then(() => {
      vm.reloadItems(vm.filter);
    });
  },

  watch: {
    'filter': {
      handler(newVal) {
        // WARNING oldVal is reference to old value, not it's deep copy!
        // TODO: For some reason this is called again after list items reload. It necessitates "equal" check.
        if (equal(this.lastLoadedFilter, newVal)) return;
        this.reloadItems(newVal);
      },
      deep: true
    },

    'loading': {
      handler(newVal) { this.$eventBus.emit('TheTopNavBar::listLoadingStatusChange', newVal, this.loaderId); },
      immediate: true,
      flush: 'post'
    }
  },

  methods: {
    /**
     * Callback called after failing to load elements of the list.
     * @param {Object} error - error message digestable by `FormatError`.
     */
    onLoadError(error) {
      console.error('NotImplementedError: ListStatic.onLoadError(error)');
    },

    /**
     * Callback called after successfuly loading elements of the list.
     * @param {Object} resp - response to load request.
     */
    onLoadSuccess(resp) {
      console.error('NotImplementedError: ListStatic.onLoadSuccess(resp)');
    },

    /**
     * Promise to load data that is necessary to initiate or render elements of this list.
     * @return {Promise}
     */
    loadAuxData() {
      return Promise.resolve([]);
    },

    /**
     * Removes all items from list.
     * @return {Promise} resolved promise with empty array.
     */
    clearOut() {
      this.items.splice(0, this.items.length);
      return Promise.resolve([]);
    },

    /**
     * Reloads items. Clears out the list and requests items from server.
     * Unless called with no filter - then simply clears list.
     * @param {Object} filter - filter
     * @return {Object} - response to load request or rejected promise with error.
     */
    reloadItems(filter) {
      this.lastLoadedFilter = Object.assign({}, filter);
      if (!filter || Object.keys(filter).length == 0) return this.clearOut();

      this.$eventBus.emit('NxnFlash::close', 'ListStatic');
      this.loading = true;
      this.displayQlParsingError = false;
      this.items.splice(0, this.items.length);
      if (!!this.beforeReload) this.beforeReload();

      var vm = this;
      this.loadItems(filter);
    },

    /**
     * Makes request to server to load items for a specified filter.
     * Calls `onLoadSuccess` on success and `onLoadError` on fail.
     * Uses and updates pagination parameters.
     * Currently still has special bahaviour for closed Incidents and IncidentTasks. TODO: move Incident/IncidentTask related code their respective lists
     * @param {Object} filter - filter that's passed to the server.
     * @return {Object} - response to load request or rejected promise with error.
     */
    loadItems(filter) {
      var request;
      var query = Object.assign({}, filter, { page: this.firstDisplayedRow / this.itemsPerPage + 1, limit: this.itemsPerPage });

      // TODO: move Incident/IncidentTask related code their respective lists
      if (filter.status == 'closed' && ['Incident', 'IncidentTask', 'EventsSeries'].indexOf(this.itemClass.klassName) >= 0) {
        // incidents api: returns only PROCESSED here, pages start at 1, incident tasks api: returns only CLOSED here
        request = this.itemClass.closed(Object.copyExcept(query, ['status']));
      } else {
        // incidents api: returns only NOT PROCESSED here, pages start at 1, incident tasks api: returns only ACTIVE here
        request = this.itemClass.query(query);
      }
      var vm = this;

      return request.then((resp) => {
        vm.totalRows = resp.total;
        vm.pageCount = vm.totalRows ? Math.ceil(vm.totalRows / vm.itemsPerPage) : 1;
        vm.loading = false;
        vm.onLoadSuccess(resp);
        return resp;
      }).catch(
        this.itemClass.authorizerFailSkipper
      ).catch((error) => {
        vm.loading = false;
        if (error.mql_parsing_error) {
          vm.displayQlParsingError = true;
          return;
        }
        vm.onLoadError(error);
        return Promise.reject(error);
      });
    },

    /**
     * Pagination menu handler. Use in template like this: `<...-pagination @change="onPaginationMenuClick" ...>`
     */
    onPaginationMenuClick(newPage) {
      this.reloadItems(this.filter);
    }
  },

  beforeUnmount() {
    if (this.updateCB) this.itemClass.removeChannelCallbacks(this.updateCB);
  }
};
</script>

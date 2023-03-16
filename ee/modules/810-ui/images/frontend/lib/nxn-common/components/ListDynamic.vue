<script>
import GlobalNxnFlash from '../services/GlobalNxnFlash.js';

export default {
  inject: ['$eventBus'],

  props: {
    filter: Object,
    loaderId: String,

    localFilter: Object, // this one is for double checking, used in itemBelongsInList
  },

  data() {
    return {
      // REQUIRES: itemClass
      items: [],
      loading: true,
      displayQlParsingError: false // Flag for special case of non-critical error - query language syntax error.
    };
  },

  watch: {
    'loading': {
      handler(newVal) { this.$eventBus.emit('TheTopNavBar::listLoadingStatusChange', newVal, this.loaderId); },
      immediate: true,
      flush: 'post'
    },

    'filter': {
       handler(newVal) {
         // WARNING oldVal is reference to old value, not it's deep copy!
         this.onFilterChange(newVal);
       },
       deep: true,
       flush: 'post'
    }
  },

  methods: {
    /**
     * Called by deep watcher on filter
     * @param {string} newVal - new filter value
     */
    onFilterChange(newVal) {
      // TODO: For some reason this is called again after list items reload. It necessitates "equal" check.
      if (equal(this.lastLoadedFilter, newVal)) return;
      this.reloadItems(newVal);
    },

    /**
     * @param {string} targetUuid - uuid to search for.
     * @return {boolean}
     */
    isInItems(targetUuid) {
      return this.items.some(function(t){ return t.uuid == targetUuid; });
    },

    /**
     * Adds item to the list.
     * @param {object} item - item to add.
     */
    add(item) {
      if (!this.isInItems(item.uuid)) {
        this.items.push(item);
        this.resort();
      } else {
        console.warn('List tried to add same item twice!');
      }
      this.afterAdd(item);
    },

    /**
     * Removes item from the list.
     * @param {string} targetUuid - uuid of item to remove.
     * @param {object} [cbVal] - argument for afterRemove callback when it shouldn't be the removed item itself.
     * @return {number} index of removed item in the list.
     */
    remove(targetUuid, cbVal) {
      var i, item, j, len, ref;

      ref = this.items;
      for (i = j = 0, len = ref.length; j < len; i = ++j) {
        if (ref[i].uuid === targetUuid) {
          item = this.items.splice(i, 1)[0];
          break;
        }
      }
      if (!!item) this.afterRemove(cbVal || item);
      return i;
    },

    /**
     * Resorts list using `sortBy(a, b)`.
     */
    resort() {
      this.items.sort(this.sortBy.bind(this));
    },

    /**
     * Creates subscription to a channel if it hasn't been done yet.
     * Only alters parameters of existing subscription if list was already subscribed.
     * WARNING: subscription doesn't wait for authorization promise resolve anymore!
     * @param {Object} subscriptionFilter - filter for subscription that is passed to the server.
     */
    subscribe(subscriptionParams) {
      if (!this.channel) {
        this.channel = this.itemClass.subscribe({ params: subscriptionParams });
      } else {
        this.itemClass.channelChangeParams(this.channel, Object.assign({}, subscriptionParams));
      }
    },

    /**
     * Removes all items from list. Deactivates subscription: makes it so that subscription doesn't add any new elements.
     * @return {Promise} resolved promise with empty array.
     */
    clearOut() {
      if (this.channel) {
        // unsubscribing can potentially destroy legit subscription, would require more testing and clearer status flow
        // subscribe to nothing
        this.channel.perform('change_params', { uuid: null });
      }
      this.items.splice(0, this.items.length);
      return Promise.resolve([]);
    },

    /**
    * Reloads items. Clears out the list and makes request to server to load items for a specified filter.
    * If called with no filter - simply clears list.
    * Sets `loading` flag to true for a duration of loading. Used for graphical purposes.
    * Sends given filter to the server but doesn't rely only on it and additionaly filters response with lists's own local filters.
    * Calls `onLoadSuccess` on success and `onLoadError` on fail.
    * @param {Object} filter - filter
    * @return {Object} - response to load request or rejected promise with error.
     */
    reloadItems(filter) {
      this.lastLoadedFilter = Object.assign({}, filter);
      if (!filter || Object.keys(filter).length == 0) return this.clearOut();

      this.$eventBus.emit('NxnFlash::close', 'ListRt');
      this.loading = true;
      this.displayQlParsingError = false;
      this.items.splice(0, this.items.length);

      this.subscribe(filter);
      var vm = this;
      return this.itemClass.query(filter).then((resp) => {
        resp.filter(function(item){ return vm.itemBelongsInList(item) && !vm.isInItems(item.uuid); }).forEach(function(item) {
          vm.items.push(item);
        });
        if (!!vm.sortBy) vm.resort();
        vm.loading = false;

        vm.onLoadSuccess();
        return resp;
      }).catch(
        this.itemClass.authorizerFailSkipper
      ).catch((error) => {
        vm.loading = false;
        if (error.mql_parsing_error) {
          vm.displayQlParsingError = true;
          return [];
        }
        return vm.onLoadError(error);
      });
    },

    /**
     * Go through all objects of class present in storage right now (or in alternativeSearchBase) and
     * create new list from the ones that satisfy `itemBelongsInList`.
     * @param {Object[]} alternativeSearchBase - alternative list of candidate objects. If given, then used instead of all objects of class.
     */
    refilterItems(alternativeSearchBase) {
      let alreadyLoading = this.loading;
      if (!alreadyLoading) this.loading = true;
      this.items.splice(0, this.items.length);
      var vm = this;
      (alternativeSearchBase || this.itemClass).filter(function(item){ return vm.itemBelongsInList(item); }).forEach(function(item) {
        vm.items.push(item);
      });
      this.resort();
      if (!alreadyLoading) this.loading = false;
    },

    /**
     * Checks if item should be in list using localFilter.
     * @param {Object} item - item to be checked.
     */
    itemBelongsInList(item) {
      if (!!this.localFilter && Object.keys(this.localFilter).length > 0) {
        var vm = this;
        return Object.keys(this.localFilter).every(function(key) {
          // WORKAROUND: for lists related to (and shown under) "subject" object
          if (key == 'except') return vm.localFilter[key] != item.uuid;

          return vm.localFilter[key] === item[key];
        });
      }
      return true;
    },

    /**
     * Activates list in correct order: channel subscriptions - first, http data load - second.
     */
    activate() {
      this.addChannelCallbacks();
      var vm = this;
      return this.loadAuxData().then(() => {
        return vm.reloadItems(vm.filter);
      });
    },

    unsubscribe() {
      if (this.channel) this.channel.unsubscribe();
    },

    /**
     * Unsubscribes from channel and removes callbacks.
     */
    destroyList() {
      this.unsubscribe();
      if (this.createCB) this.itemClass.removeChannelCallbacks(this.createCB);
      if (this.updateCB) this.itemClass.removeChannelCallbacks(this.updateCB);
      if (this.deleteCB) this.itemClass.removeChannelCallbacks(this.deleteCB);
      this.items.splice(0, this.items.length);
    },

    // CHANNEL CALLBACKS
    /**
     * Used to ignore callbacks of other channels.
     * As all callbacks of a class for a certain message type (except the ones excluded by `dontCall` argument) are all called at once,
     * any callback that wants to be called only for certain channel has to ensure it itself.
     * @param {Object} kwargs - extraKwargs passed from channel handlers.
     */
    shouldIgnoreCallback(kwargs) {
      return !!kwargs && !!kwargs.channel && kwargs.channel !== this.channel;
    },

    /**
     * Activates this list's channel callbacks.
     */
    addChannelCallbacks() {
      var vm = this;

      this.createCB = this.itemClass.addChannelCallback('create', function(item, extraKwargs) {
        if (vm.shouldIgnoreCallback(extraKwargs)) return;
        vm.onCreate(item);
      });
      this.updateCB = this.itemClass.addChannelCallback('update', function(item, oldVal, extraKwargs) {
        if (vm.shouldIgnoreCallback(extraKwargs)) return;
        vm.onUpdate(item, oldVal);
      });
      this.deleteCB = this.itemClass.addChannelCallback('delete', function(deletedItem, extraKwargs) {
        if (vm.shouldIgnoreCallback(extraKwargs)) return;
        vm.onDelete(deletedItem, extraKwargs);
      });
    },

    /**
     * Callback called after 'create' channel message.
     * From subscription's POV, new item was created, therefore it is a candidate for a new item in the list.
     * @param {Object} item - Instance of a `itemClass`.
     */
    onCreate(item) {
      if (this.itemBelongsInList(item)) {
        this.add(item);
      }
    },

    /**
     * Callback called after 'update' channel message.
     * From subscription's POV, item change didn't change it's status within subscription conditions.
     * But, if list has a localFilter, then list uses it for a secondary check.
     * @param {Object} item - Instance of a `itemClass`.
     * @param {Object} oldVal - NOT an instance of a `itemClass` and doen't have it's methods! TODO: rename to `preChangeCopy` as it is named in NxnDB code.
     */
    onUpdate(item, oldVal) {
      if (this.itemBelongsInList(item) && !this.isInItems(item.uuid)) {
        this.add(item);
      } else if (!this.itemBelongsInList(item) && this.isInItems(item.uuid)) {
        this.remove(item.uuid);
      } else {
        this.onItemDataUpdate(item, oldVal);
      }
    },

    /**
     * Callback called after 'delete' channel message.
     * From subscription's POV, item was deleted, therefore it is should be removed from list.
     * @param {Object} item - Instance of a `itemClass`.
     */
    onDelete(deletedItem) {
      this.remove(deletedItem.uuid);
    },

    // REQUIRED CALLBACKS
    /**
     * Callback called after failing to load elements of the list.
     * @param {Object} error - error message digestable by `FormatError`.
     */
    onLoadError(error) {
      console.error('NotImplementedError: ListDynamic.onLoadError(error)');
    },

    // OPTIONAL CALLBACKS
    /**
     * Promise to load data that is necessary to initiate or render elements of this list.
     * @return {Promise}
     */
    loadAuxData() {
      return Promise.resolve([]);
    },

    /**
     * Callback called after successfuly loading elements of the list.
     */
    onLoadSuccess() {
    },

    /**
     * Callback called after change in item that resulted neither in adding item to the list nor removing it.
     * @param {Object} item - Updated item. Instance of a `itemClass`.
     * @param {Object} oldVal - NOT an instance of a `itemClass` and doen't have it's methods! TODO: rename to `preChangeCopy` as it is named in NxnDB code.
     */
    onItemDataUpdate(item, oldVal) {
    },

    /**
     * Callback called after change in item that resulted neither in adding item to the list nor removing it.
     * @param {Object} item - Added item. Instance of a `itemClass`.
     */
    afterAdd(item) {
    },

    /**
     * Callback called after change in item that resulted neither in adding item to the list nor removing it.
     * @param {Object} item - Removed item. Instance of a `itemClass`.
     */
    afterRemove(item) {
    },

    /**
     * Comparison function that used in `resort()`.
     */
    sortBy(a, b) {
      return b.created_at - a.created_at;
    }
  }
};
</script>

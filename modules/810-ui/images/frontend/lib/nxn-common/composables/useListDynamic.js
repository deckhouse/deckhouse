import GlobalNxnFlash from '../services/GlobalNxnFlash.js';
import { ref, reactive, inject, onBeforeUnmount, watch } from "vue";

export default function useListDynamic(itemClass, loaderId, filter, localFilter) {
  const $eventBus = inject('$eventBus');
  const route = useRoute();

  /*
  props: {
    filter: Object,
    loaderId: String,
    localFilter: Object, // this one is for double checking, used in itemBelongsInList
  }
  */

  const items = ref([])
  const isLoading = ref(true)

  const createCB = undefined
  const updateCB = undefined
  const deleteCB = undefined

  watch(
    isLoading,
    (newVal) => { $eventBus.emit('TheTopNavBar::listLoadingStatusChange', newVal, loaderId); },
    {
      immediate: true,
      flush: 'post'
    }
  )

  watch(
    filter,
    (newVal) => { onFilterChange(newVal); }, // WARNING oldVal is reference to old value, not it's deep copy!
    {
      deep: true,
      flush: 'post'
   }
  )

  /**
   * Called by deep watcher on filter
   * @param {string} newVal - new filter value
   */
  function onFilterChange(newVal) {
    // TODO: For some reason this is called again after list items reload. It necessitates "equal" check.
    if (equal(lastLoadedFilter, newVal)) return;
    reloadItems(newVal);
  }

  /**
   * @param {string} targetUuid - uuid to search for.
   * @return {boolean}
   */
  function isInItems(targetUuid) {
    return items.some(function(t){ return t.uuid == targetUuid; });
  }

  /**
   * Adds item to the list.
   * @param {object} item - item to add.
   */
  function add(item) {
    if (!isInItems(item.uuid)) {
      items.push(item);
      resort();
    } else {
      console.warn('List tried to add same item twice!');
    }
    afterAdd(item);
  }

  /**
   * Removes item from the list.
   * @param {string} targetUuid - uuid of item to remove.
   * @param {object} [cbVal] - argument for afterRemove callback when it shouldn't be the removed item itself.
   * @return {number} index of removed item in the list.
   */
  function remove(targetUuid, cbVal) {
    var i, item, j, len, ref;

    ref = items;
    for (i = j = 0, len = ref.length; j < len; i = ++j) {
      if (ref[i].uuid === targetUuid) {
        item = items.splice(i, 1)[0];
        break;
      }
    }
    if (!!item) afterRemove(cbVal || item);
    return i;
  }

  /**
   * Resorts list using `sortBy(a, b)`.
   */
  function resort() {
    items.sort(sortBy.bind(this));
  }

  /**
   * Creates subscription to a channel if it hasn't been done yet.
   * Only alters parameters of existing subscription if list was already subscribed.
   * WARNING: subscription doesn't wait for authorization promise resolve anymore!
   * @param {Object} subscriptionFilter - filter for subscription that is passed to the server.
   */
  function subscribe(subscriptionParams) {
    if (!channel) {
      channel = itemClass.subscribe({ params: subscriptionParams });
    } else {
      itemClass.channelChangeParams(channel, Object.assign({}, subscriptionParams));
    }
  }

  /**
   * Removes all items from list. Deactivates subscription: makes it so that subscription doesn't add any new elements.
   * @return {Promise} resolved promise with empty array.
   */
  function clearOut() {
    if (channel) {
      // unsubscribing can potentially destroy legit subscription, would require more testing and clearer status flow
      // subscribe to nothing
      channel.perform('change_params', { uuid: null });
    }
    items.splice(0, items.length);
    return Promise.resolve([]);
  }

  /**
  * Reloads items. Clears out the list and makes request to server to load items for a specified filter.
  * If called with no filter - simply clears list.
  * Sets `isLoading` flag to true for a duration of isLoading. Used for graphical purposes.
  * Sends given filter to the server but doesn't rely only on it and additionaly filters response with lists's own local filters.
  * Calls `onLoadSuccess` on success and `onLoadError` on fail.
  * @param {Object} filter - filter
  * @return {Object} - response to load request or rejected promise with error.
    */
  function reloadItems(filter) {
    lastLoadedFilter = Object.assign({}, filter);
    if (!filter || Object.keys(filter).length == 0) return clearOut();

    $eventBus.emit('NxnFlash::close', 'ListRt');
    isLoading = true;
    items.splice(0, items.length);

    subscribe(filter);

    return itemClass.query(filter).then((resp) => {
      resp.filter(function(item){ return itemBelongsInList(item) && !isInItems(item.uuid); }).forEach(function(item) {
        items.push(item);
      });
      if (!!sortBy) resort();
      isLoading = false;

      onLoadSuccess();
      return resp;
    }).catch(
      itemClass.authorizerFailSkipper
    ).catch((error) => {
      isLoading = false;
      return onLoadError(error);
    });
  }

  /**
   * Go through all objects of class present in storage right now (or in alternativeSearchBase) and
   * create new list from the ones that satisfy `itemBelongsInList`.
   * @param {Object[]} alternativeSearchBase - alternative list of candidate objects. If given, then used instead of all objects of class.
   */
  function refilterItems(alternativeSearchBase) {
    let alreadyLoading = isLoading;
    if (!alreadyLoading) isLoading = true;
    items.splice(0, items.length);

    (alternativeSearchBase || itemClass).filter(function(item){ return itemBelongsInList(item); }).forEach(function(item) {
      items.push(item);
    });
    resort();
    if (!alreadyLoading) isLoading = false;
  }

  /**
   * Checks if item should be in list using localFilter.
   * @param {Object} item - item to be checked.
   */
  function itemBelongsInList(item) {
    if (!!localFilter && Object.keys(localFilter).length > 0) {

      return Object.keys(localFilter).every(function(key) {
        // WORKAROUND: for lists related to (and shown under) "subject" object
        if (key == 'except') return localFilter[key] != item.uuid;

        return localFilter[key] === item[key];
      });
    }
    return true;
  }

  /**
   * Activates list in correct order: channel subscriptions - first, http data load - second.
   */
  function activate() {
    addChannelCallbacks();
    return loadAuxData().then(() => {
      return reloadItems(filter);
    });
  }

  function unsubscribe() {
    if (channel) channel.unsubscribe();
  }

  /**
   * Unsubscribes from channel and removes callbacks.
   */
  function destroyList() {
    unsubscribe();
    if (createCB) itemClass.removeChannelCallbacks(createCB);
    if (updateCB) itemClass.removeChannelCallbacks(updateCB);
    if (deleteCB) itemClass.removeChannelCallbacks(deleteCB);
    items.splice(0, items.length);
  }

  // CHANNEL CALLBACKS
  /**
   * Used to ignore callbacks of other channels.
   * As all callbacks of a class for a certain message type (except the ones excluded by `dontCall` argument) are all called at once,
   * any callback that wants to be called only for certain channel has to ensure it itself.
   * @param {Object} kwargs - extraKwargs passed from channel handlers.
   */
  function shouldIgnoreCallback(kwargs) {
    return !!kwargs && !!kwargs.channel && kwargs.channel !== channel;
  }

  /**
   * Activates this list's channel callbacks.
   */
  function addChannelCallbacks() {
    createCB = itemClass.addChannelCallback('create', function(item, extraKwargs) {
      if (shouldIgnoreCallback(extraKwargs)) return;
      onCreate(item);
    });
    updateCB = itemClass.addChannelCallback('update', function(item, oldVal, extraKwargs) {
      if (shouldIgnoreCallback(extraKwargs)) return;
      onUpdate(item, oldVal);
    });
    deleteCB = itemClass.addChannelCallback('delete', function(deletedItem, extraKwargs) {
      if (shouldIgnoreCallback(extraKwargs)) return;
      onDelete(deletedItem, extraKwargs);
    });
  }

  /**
   * Callback called after 'create' channel message.
   * From subscription's POV, new item was created, therefore it is a candidate for a new item in the list.
   * @param {Object} item - Instance of a `itemClass`.
   */
  function onCreate(item) {
    if (itemBelongsInList(item)) {
      add(item);
    }
  }

  /**
   * Callback called after 'update' channel message.
   * From subscription's POV, item change didn't change it's status within subscription conditions.
   * But, if list has a localFilter, then list uses it for a secondary check.
   * @param {Object} item - Instance of a `itemClass`.
   * @param {Object} oldVal - NOT an instance of a `itemClass` and doen't have it's methods! TODO: rename to `preChangeCopy` as it is named in NxnDB code.
   */
  function onUpdate(item, oldVal) {
    if (itemBelongsInList(item) && !isInItems(item.uuid)) {
      add(item);
    } else if (!itemBelongsInList(item) && isInItems(item.uuid)) {
      remove(item.uuid);
    } else {
      onItemDataUpdate(item, oldVal);
    }
  }

  /**
   * Callback called after 'delete' channel message.
   * From subscription's POV, item was deleted, therefore it is should be removed from list.
   * @param {Object} item - Instance of a `itemClass`.
   */
  function onDelete(deletedItem) {
    remove(deletedItem.uuid);
  }

  // REQUIRED CALLBACKS
  /**
   * Callback called after failing to load elements of the list.
   * @param {Object} error - error message digestable by `FormatError`.
   */
  function onLoadError(error) {
    console.error('NotImplementedError: ListDynamic.onLoadError(error)');
  }

  // OPTIONAL CALLBACKS
  /**
   * Promise to load data that is necessary to initiate or render elements of this list.
   * @return {Promise}
   */
  function loadAuxData() {
    return Promise.resolve([]);
  }

  /**
   * Callback called after successfuly isLoading elements of the list.
   */
  function onLoadSuccess() {
  }

  /**
   * Callback called after change in item that resulted neither in adding item to the list nor removing it.
   * @param {Object} item - Updated item. Instance of a `itemClass`.
   * @param {Object} oldVal - NOT an instance of a `itemClass` and doen't have it's methods! TODO: rename to `preChangeCopy` as it is named in NxnDB code.
   */
  function onItemDataUpdate(item, oldVal) {
  }

  /**
   * Callback called after change in item that resulted neither in adding item to the list nor removing it.
   * @param {Object} item - Added item. Instance of a `itemClass`.
   */
  function afterAdd(item) {
  }

  /**
   * Callback called after change in item that resulted neither in adding item to the list nor removing it.
   * @param {Object} item - Removed item. Instance of a `itemClass`.
   */
  function afterRemove(item) {
  }

  /**
   * Comparison function that used in `resort()`.
   */
  function sortBy(a, b) {
    return b.created_at - a.created_at;
  }
  
  return {
    items,
    isLoading
  };
};
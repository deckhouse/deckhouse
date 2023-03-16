// import GlobalNxnFlash from '../services/GlobalNxnFlash.js';
import { ref, reactive, inject, onBeforeUnmount, watch } from "vue";
import { useRoute } from "vue-router";
const equal = import("fast-deep-equal");
import type NxnResourceWs from "../models/NxnResourceWs";

interface useListDynamicCallbacks<T> {
  // REQUIRED CALLBACKS
  sortBy(a: T, b: T): number;

  /**
   * Callback called after failing to load elements of the list.
   * @param {Object} error - error message digestable by `FormatError`.
   */
  onLoadError(error: any): void;

  // OPTIONAL CALLBACKS
  /**
   * Promise to load data that is necessary to initiate or render elements of this list.
   * @return {Promise}
   */
  loadAuxData?(): Promise<null>;

  /**
   * Callback called after successfuly isLoading elements of the list.
   */
  onLoadSuccess?(): void;

  /**
   * Callback called after change in item that resulted neither in adding item to the list nor removing it.
   * @param {Object} item - Updated item. Instance of a `itemClass`.
   * @param {Object} oldVal - NOT an instance of a `itemClass` and doen't have it's methods! TODO: rename to `preChangeCopy` as it is named in NxnDB code.
   */
  onItemDataUpdate?(item: T, oldVal: object): void;

  /**
   * Callback called after change in item that resulted neither in adding item to the list nor removing it.
   * @param {Object} item - Added item. Instance of a `itemClass`.
   */
  afterAdd?(item: T): void;

  /**
   * Callback called after change in item that resulted neither in adding item to the list nor removing it.
   * @param {Object} item - Removed item. Instance of a `itemClass`.
   */
  afterRemove?(item: T | object): void;
}

interface ChannelCBsKwargs {
  channel: any;
}

interface Filter {
  [key: string]: string | object;
}

export default function useListDynamic<T extends NxnResourceWs>(
  itemClass: typeof NxnResourceWs,
  cb: useListDynamicCallbacks<T>,
  filter: Filter,
  localFilter: Filter | null = null, // this one is for double checking, used in itemBelongsInList
  noFilters: Boolean = false
) {
  // const $eventBus = inject('$eventBus');
  const route = useRoute();

  const items = reactive<Array<T>>([]);
  const isLoading = ref<Boolean>(true);
  let lastLoadedFilter: Filter;
  let channel: any = undefined;

  let createCB: Function | null = null;
  let updateCB: Function | null = null;
  let deleteCB: Function | null = null;

  watch(
    isLoading,
    (newVal) => {
      console.log(`${newVal ? "STARTED" : "FINISHED"} LOADING`);
    },
    {
      immediate: true,
      flush: "post",
    }
  );

  if (!noFilters) {
    watch(
      filter,
      (newVal) => {
        onFilterChange(newVal);
      }, // WARNING oldVal is reference to old value, not it's deep copy!
      {
        // deep: true,
        flush: "post",
      }
    );
  }

  /**
   * Called by deep watcher on filter
   * @param {object} newVal - new filter value
   */
  function onFilterChange(newVal: object) {
    if (equal(lastLoadedFilter, newVal)) return;
    reloadItems();
  }

  /**
   * @param {string} targetKey - key to search for.
   * @return {boolean}
   */
  function isInItems(targetKey: string): Boolean {
    return items.some((item: T) => {
      return item.primaryKey() == targetKey;
    });
  }

  /**
   * Adds item to the list.
   * @param {object} item - item to add.
   */
  function add(item: T) {
    if (!isInItems(item.primaryKey())) {
      items.push(item);
      resort();
    } else {
      console.warn("List tried to add same item twice!");
    }
    if (cb.afterAdd) cb.afterAdd(item);
  }

  /**
   * Removes item from the list.
   * @param {string} targetKey - key of item to remove.
   * @param {object} [cbVal] - argument for afterRemove callback when it shouldn't be the removed item itself.
   * @return {number} index of removed item in the list.
   */
  function remove(targetKey: string, cbVal?: object): number {
    let i, item, j, len;

    const ref = items;
    for (i = j = 0, len = ref.length; j < len; i = ++j) {
      if (ref[i].primaryKey() === targetKey) {
        item = items.splice(i, 1)[0];
        break;
      }
    }
    if (!!item && !!cb.afterRemove) cb.afterRemove(cbVal || item);
    return i;
  }

  /**
   * Resorts list using `sortBy(a, b)`.
   */
  function resort() {
    items.sort(cb.sortBy);
  }

  /**
   * Creates subscription to a channel if it hasn't been done yet.
   * Only alters parameters of existing subscription if list was already subscribed.
   * WARNING: subscription doesn't wait for authorization promise resolve anymore!
   * @param {Object} subscriptionParams - filter for subscription that is passed to the server.
   */
  function subscribe() {
    if (!channel) {
      channel = itemClass.subscribe({ params: filter });
    } else {
      itemClass.channelChangeParams(channel, Object.assign({}, filter));
    }
  }

  /**
   * Removes all items from list. Deactivates subscription: makes it so that subscription doesn't add any new elements.
   * @return {Promise} resolved promise with empty array.
   */
  function clearOut(): Promise<null> {
    if (channel) {
      // unsubscribing can potentially destroy legit subscription, would require more testing and clearer status flow
      // subscribe to nothing
      channel.perform("change_params", { key: null });
    }
    items.splice(0, items.length);
    return Promise.resolve(null);
  }

  /**
   * Reloads items Clears out the list and makes request to server to load items for a specified filter.
   * If called with no filter - simply clears list.
   * Sets `isLoading` flag to true for a duration of isLoading. Used for graphical purposes.
   * Sends given filter to the server but doesn't rely only on it and additionaly filters response with lists's own local filters.
   * Calls `onLoadSuccess` on success and `onLoadError` on fail.
   * @param {Object} filter - filter
   * @return {Object} - response to load request or rejected promise with error.
   */
  function reloadItems() {
    lastLoadedFilter = Object.assign({}, filter) as Filter;

    isLoading.value = true;
    items.splice(0, items.length);

    subscribe();

    return itemClass
      .query(filter)
      .then((resp: any) => {
        resp
          .filter((item: T) => {
            return itemBelongsInList(item) && !isInItems(item.primaryKey());
          })
          .forEach((item: T) => {
            items.push(item);
          });
        resort();
        isLoading.value = false;

        if (cb.onLoadSuccess) cb.onLoadSuccess();
        return resp;
      })
      .catch((error: any) => {
        console.error(error);
        isLoading.value = false;
        return cb.onLoadError(error);
      });
  }

  /**
   * Go through all objects of class present in storage right now (or in alternativeSearchBase) and
   * create new list from the ones that satisfy `itemBelongsInList`.
   * @param {Object[]} alternativeSearchBase - alternative list of candidate objects. If given, then used instead of all objects of class.
   */
  function refilterItems() {
    const alreadyLoading = isLoading.value;
    if (!alreadyLoading) isLoading.value = true;
    items.splice(0, items.length);

    itemClass
      .filter((item: T) => {
        return itemBelongsInList(item);
      })
      .forEach((item: T) => {
        items.push(item);
      });
    resort();
    if (!alreadyLoading) isLoading.value = false;
  }

  /**
   * Checks if item should be in list using localFilter.
   * @param {Object} item - item to be checked.
   */
  function itemBelongsInList(item: T) {
    if (!!localFilter && Object.keys(localFilter).length > 0) {
      return Object.keys(localFilter).every((key: string) => {
        // WORKAROUND: for lists related to (and shown under) "subject" object
        if (key == "except") return localFilter[key] != item.primaryKey();

        return localFilter[key] === item[key];
      });
    }
    return true;
  }

  /**
   * Activates list in correct order: channel subscriptions - first, http data load - second.
   */
  function activate(): Promise<object> {
    if (!noFilters) addChannelCallbacks();
    return (cb.loadAuxData ? cb.loadAuxData() : Promise.resolve(null)).then(() => {
      return reloadItems();
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
  function shouldIgnoreCallback(kwargs: ChannelCBsKwargs): Boolean {
    return !!kwargs && !!kwargs.channel && kwargs.channel !== channel;
  }

  /**
   * Activates this list's channel callbacks.
   */
  function addChannelCallbacks() {
    createCB = itemClass.addChannelCallback("create", function (item: T, extraKwargs: ChannelCBsKwargs) {
      if (shouldIgnoreCallback(extraKwargs)) return;
      onCreate(item);
    });

    updateCB = itemClass.addChannelCallback("update", function (item: T, oldVal: object, extraKwargs: ChannelCBsKwargs) {
      if (shouldIgnoreCallback(extraKwargs)) return;
      onUpdate(item, oldVal);
    });

    deleteCB = itemClass.addChannelCallback("delete", function (deletedItem: T, extraKwargs: ChannelCBsKwargs) {
      if (shouldIgnoreCallback(extraKwargs)) return;
      onDelete(deletedItem);
    });
  }

  /**
   * Callback called after 'create' channel message.
   * From subscription's POV, new item was created, therefore it is a candidate for a new item in the list.
   * @param {Object} item - Instance of a `itemClass`.
   */
  function onCreate(item: T) {
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
  function onUpdate(item: T, oldVal: object) {
    if (itemBelongsInList(item) && !isInItems(item.primaryKey())) {
      add(item);
    } else if (!itemBelongsInList(item) && isInItems(item.primaryKey())) {
      remove(item.primaryKey());
    } else if (cb.onItemDataUpdate) {
      cb.onItemDataUpdate(item, oldVal);
    }
  }

  /**
   * Callback called after 'delete' channel message.
   * From subscription's POV, item was deleted, therefore it is should be removed from list.
   * @param {Object} item - Instance of a `itemClass`.
   */
  function onDelete(deletedItem: T) {
    remove(deletedItem.primaryKey());
  }
  return {
    items,
    isLoading,
    activate,
    destroyList,
    clearOut,
  };
}

import { reactive, watch, computed, type ComputedRef, type Ref, ref } from "vue";
import useListDynamic from "@lib/nxn-common/composables/useListDynamic";
import { useRoute } from "vue-router";

import Discovery from "@/models/Discovery";
import DeckhouseRelease from "@/models/DeckhouseRelease";
import NodeGroup from "@/models/NodeGroup";
import Node from "@/models/Node";
import type { InstanceClassesTypes } from "@/models/instanceclasses";
import DeckhouseModuleSettings from "@/models/DeckhouseModuleSettings";

interface useLoadAll {
  lists: typeof lists;
  deckhouseSettings: Ref<DeckhouseModuleSettings | undefined>;
  isLoading: ComputedRef<boolean>;
}

const sortBy = ref<string | undefined>();

const lists = reactive<{ [key: string]: ReturnType<typeof useListDynamic> }>({});
const deckhouseSettings = ref<DeckhouseModuleSettings>();

const deckhouseSettingsLoading = ref(false);
const isLoading = computed<boolean>(() => !!Object.values(lists).find((l) => l.isLoading) || deckhouseSettingsLoading.value);

const isWatchingRoute = ref(false);

function loadReleases() {
  lists.releases = useListDynamic<DeckhouseRelease>(
    DeckhouseRelease,
    {
      onLoadError: loadError,
      sortBy: (a: DeckhouseRelease, b: DeckhouseRelease) => {
        return Date.parse(b.metadata.creationTimestamp) - Date.parse(a.metadata.creationTimestamp);
      },
    },
    {}
  );

  lists.releases.activate();

  deckhouseSettingsLoading.value = true;
  DeckhouseModuleSettings.get().then((res: DeckhouseModuleSettings) => {
    deckhouseSettings.value = res;
    deckhouseSettingsLoading.value = false;
    DeckhouseModuleSettings.subscribe({ klassChannel: true });
  });
}

function loadError(error: any) {
  console.error("LOAD ERROR!", error);
  let errorText: string;
  switch (error.response.status) {
    case 404: {
      errorText = "Не найдено.";
      break;
    }
    default: {
      errorText = "Что-то пошло не так.";
    }
  }
  const loadError = {
    code: error.response.status,
    text: errorText,
  };
  // TODO:
}

function loadNodeControl() {
  console.log("useLoadAll:loadNodeControl");
  lists.nodeGroups = useListDynamic<NodeGroup>(
    NodeGroup,
    {
      sortBy: (a, b) => {
        console.log("SORTING NODE GROUPS!", sortBy.value);

        switch (sortBy.value) {
          case "creationTimestamp": {
            return Date.parse(b.creationTimestamp) - Date.parse(a.creationTimestamp);
          }
          default: {
            return String(a.metadata.name).localeCompare(String(b.metadata.name));
          }
        }
      },
      onLoadError: loadError,
    },
    {}
  );
  lists.nodes = useListDynamic<Node>(
    Node,
    {
      sortBy: (a, b) => {
        switch (sortBy.value) {
          case "creationTimestamp": {
            return Date.parse(b.metadata.creationTimestamp) - Date.parse(a.metadata.creationTimestamp);
          }
          default: {
            return String(a.metadata.name).localeCompare(String(b.metadata.name));
          }
        }
      },
      onLoadError: loadError,
    },
    {}
  );

  lists.instanceClasses = useListDynamic<InstanceClassesTypes>(
    Discovery.get().instanceClassKlass,
    {
      sortBy: (a, b) => {
        switch (sortBy.value) {
          case "creationTimestamp": {
            return Date.parse(b.creationTimestamp) - Date.parse(a.creationTimestamp);
          }
          default: {
            return String(a.name).localeCompare(String(b.name));
          }
        }
      },
      onLoadError: loadError,
    },
    {}
  );

  lists.nodeGroups.activate();
  lists.nodes.activate();
  lists.instanceClasses.activate();
}

function unloadNodeControl() {
  console.log("useLoadAll:UNloadNodeControl");
  lists.nodeGroups.destroyList();
  lists.nodes.destroyList();
  lists.instanceClasses.destroyList();

  delete lists.nodeGroups;
  delete lists.nodes;
  delete lists.instanceClasses;
}

function needsNodeControlLists(routeName: string) {
  return !![/^Node[a-zA-Z]+$/, /^InstanceClass[a-zA-Z]+$/].find((r) => r.test(routeName));
}

function setupRouteWatcher() {
  if (isWatchingRoute.value) return;
  loadReleases(); // TODO: For every route?

  const route = useRoute();
  watch(
    () => route?.name?.toString(),
    (newVal: string, oldVal: string | undefined): void => {
      if (needsNodeControlLists(newVal) && (!oldVal || !needsNodeControlLists(oldVal))) {
        loadNodeControl();
      } else if (!needsNodeControlLists(newVal) && !!oldVal && needsNodeControlLists(oldVal)) {
        unloadNodeControl();
      }
    },
    {
      immediate: true,
    }
  );

  watch(
    () => route?.query?.sortBy?.toString(),
    (value) => {
      sortBy.value = value;

      Object.values(lists).forEach((l) => l.resort()); // TODO: do not resort every list
    },
    {
      immediate: true,
    }
  );

  isWatchingRoute.value = true;
}

/**
 * Automagical function that loads all data for current section (based on route)
 * @param {funciton} cb - callback that will be called after all data was loaded.
 */
export default function useLoadAll(cb?: (l: Partial<useLoadAll>) => void): useLoadAll {
  setupRouteWatcher();

  watch(
    isLoading,
    (value: boolean) => {
      if (!value && cb) cb({ lists, deckhouseSettings });
    },
    { immediate: true } // Should run immediatly in case we have already loaded lists in previous composable calls
  );

  return { lists, deckhouseSettings, isLoading };
}

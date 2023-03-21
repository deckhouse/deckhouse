<template>
  <nav class="hs-accordion-group py-6 w-full flex flex-col flex-wrap" data-hs-accordion-always-open>
    <ul>
      <SidebarNavItem v-for="(item, idx) in items" :key="idx" :item="item" />
    </ul>
  </nav>
</template>

<script setup lang="ts">
import SidebarNavItem from "./SidebarNavItem.vue";
import { ref, reactive, onBeforeUnmount, watch } from "vue";
import { useRoute } from "vue-router";

import useListDynamic from "@lib/nxn-common/composables/useListDynamic";
import Discovery from "@/models/Discovery";
import DeckhouseRelease from "@/models/DeckhouseRelease";
import NodeGroup from "@/models/NodeGroup";
import Node from "@/models/Node";
import InstanceClassBase from "@/models/instanceclasses/InstanceClassBase";

const counts = {
  releaseItems: ref<number | null>(null),
  nodeGroups: ref<number | null>(null),
  nodes: ref<number | null>(null),
  instanceClasses: ref<number | null>(null),
};

const items = reactive([
  {
    title: "Обновления",
    icon: "IconOverview",
    routeNames: ["Home", "DeckhouseSettings"],
    badge: counts.releaseItems,
  },
  {
    title: "Управление узлами",
    icon: "IconOverview",
    routeNames: ["NodeGroupList"],
    children: [
      {
        title: "Группы узлов",
        routeNames: ["NodeGroupList", "NodeGroupShow", "NodeGroupEdit", "NodeShow", "NodeEdit"],
        badge: counts.nodeGroups,
      },
      {
        title: "Классы машин",
        routeNames: ["InstanceClassesList", "InstanceClassShow", "InstanceClassEdit", "InstanceClassNew"],
        badge: counts.instanceClasses,
      },
      {
        title: "Узлы всех групп",
        routeNames: ["NodeListAll"],
        badge: counts.nodes,
      },
    ],
  },
]);

function countLoadError(error: any) {
  console.error("Failed to load counts: " + JSON.stringify(error));
}

function resetReleaseItemsCount() {
  counts.releaseItems.value = releaseItemsList.items.length;
}
const releaseItemsList = useListDynamic<DeckhouseRelease>(
  DeckhouseRelease,
  {
    onLoadSuccess: resetReleaseItemsCount,
    afterAdd: resetReleaseItemsCount,
    afterRemove: resetReleaseItemsCount,
    onLoadError: countLoadError,
  },
  {}
);
releaseItemsList.activate();

let nodeGroupsList: ReturnType<typeof useListDynamic<NodeGroup>>;
let nodesList: ReturnType<typeof useListDynamic<Node>>;
let instanceClassesList: ReturnType<typeof useListDynamic<InstanceClassBase>>;
function resetNodeGroupsCount() {
  counts.nodeGroups.value = nodeGroupsList.items.length;
}
function resetNodesCount() {
  counts.nodes.value = nodesList.items.length;
}
function resetInstanceClassesCount() {
  counts.instanceClasses.value = instanceClassesList.items.length;
}
function activateNodeControlCounters() {
  console.log("SIDEBAR:activateNodeControlCounters");
  nodeGroupsList = useListDynamic<NodeGroup>(
    NodeGroup,
    { onLoadSuccess: resetNodeGroupsCount, afterAdd: resetNodeGroupsCount, afterRemove: resetNodeGroupsCount, onLoadError: countLoadError },
    {}
  );
  nodesList = useListDynamic<Node>(
    Node,
    { onLoadSuccess: resetNodesCount, afterAdd: resetNodesCount, afterRemove: resetNodesCount, onLoadError: countLoadError },
    {}
  );

  instanceClassesList = useListDynamic<InstanceClassBase>(
    Discovery.get().instanceClassKlass,
    {
      onLoadSuccess: resetInstanceClassesCount,
      afterAdd: resetInstanceClassesCount,
      afterRemove: resetInstanceClassesCount,
      onLoadError: countLoadError,
    },
    {}
  );

  nodeGroupsList.activate();
  nodesList.activate();
  instanceClassesList.activate();
}

function deactivateNodeControlCounters() {
  console.log("SIDEBAR:DEactivateNodeControlCounters");
  nodeGroupsList.destroyList();
  nodesList.destroyList();
  instanceClassesList.destroyList();
  counts.nodeGroups.value = null;
  counts.nodes.value = null;
  counts.instanceClasses.value = null;
}

function needsNodeControlCounters(routeName: string) {
  return !![/^Node[a-zA-Z]+$/, /^InstanceClass[a-zA-Z]+$/].find((r) => r.test(routeName));
}

const route = useRoute();
watch(
  () => route.name?.toString(),
  (newVal: string, oldVal: string | undefined): void => {
    if (needsNodeControlCounters(newVal) && (!oldVal || !needsNodeControlCounters(oldVal))) {
      activateNodeControlCounters();
    } else if (!needsNodeControlCounters(newVal) && !!oldVal && needsNodeControlCounters(oldVal)) {
      deactivateNodeControlCounters();
    }
  },
  {
    immediate: true,
  }
);
</script>

<template>
  <template v-if="!list.isLoading.value">
    <NodeListItem v-for="item in list.items" :key="item.metadata.name" :item="item"></NodeListItem>
  </template>
  <CardBlock v-if="list.isLoading.value" :content-loading="true"></CardBlock>
</template>

<script setup lang="ts">
import { reactive, watch, onBeforeUnmount } from "vue";
import { useRoute } from "vue-router";

import useListDynamic from "@lib/nxn-common/composables/useListDynamic";

import Node from "@/models/Node";

import CardBlock from "@/components/common/card/CardBlock.vue";
import NodeListItem from "@/components/node/NodeListItem.vue";

const props = defineProps({
  sortBy: {
    type: String,
    required: true,
  },
});

watch(
  () => props.sortBy,
  () => list.resort()
);

const route = useRoute();

const filter = reactive<any>({});
const localFilter = reactive<any>({});
if (route.params.ng_name) {
  // filter["node.deckhouse.io/group"] = route.params.ng_name;
  // localFilter["node.deckhouse.io/group"] = route.params.ng_name;
  localFilter.nodeGroupName = route.params.ng_name;
}

const emit = defineEmits<{ (e: "set-count", value: number): void }>();
function resetCount() { emit("set-count", list.items.length); }

const list = useListDynamic<Node>(
  Node,
  {
    onLoadSuccess: resetCount,
    afterAdd: resetCount,
    afterRemove: resetCount,

    sortBy: (a: Node, b: Node) => {
      switch (props.sortBy) {
        case "name": {
          return String(a.metadata.name).localeCompare(String(b.metadata.name));
        }
        default: {
          return Date.parse(b.metadata.creationTimestamp) - Date.parse(a.metadata.creationTimestamp);
        }
      }
    },

    onLoadError: (error: any) => {
      console.error("NotImplementedError: ReleaseItemsList.onLoadError: " + JSON.stringify(error));
    },
  },
  filter,
  localFilter
);

list.activate();
onBeforeUnmount(() => list.destroyList() );
</script>

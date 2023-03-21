<template>
  <template v-if="!list.isLoading.value">
    <NodeGroupListItem v-for="item in list.items" :key="item.metadata.name" :item="item" ></NodeGroupListItem>
  </template>
  <CardBlock v-if="list.isLoading.value" :content-loading="true"></CardBlock>
</template>

<script setup lang="ts">
import { reactive, watch, onBeforeUnmount } from "vue";
import { useRoute } from "vue-router";

import useListDynamic from "@lib/nxn-common/composables/useListDynamic";

import NodeGroup from "@/models/NodeGroup";

import CardBlock from "@/components/common/card/CardBlock.vue";
import NodeGroupListItem from "@/components/node_group/NodeGroupListItem.vue";

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

const filter = reactive({});
const list = useListDynamic<NodeGroup>(
  NodeGroup,
  {
    sortBy: (a: NodeGroup, b: NodeGroup) => {
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
      console.error("NotImplementedError: NodeGroupList.onLoadError: " + JSON.stringify(error));
    },
  },
  filter
);

list.activate();
onBeforeUnmount(() => list.destroyList() );
</script>

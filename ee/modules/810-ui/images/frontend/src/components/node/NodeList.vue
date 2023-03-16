<template>
  <template v-if="!list.isLoading.value">
    <NodeListItem v-for="item in list.items" :key="item.metadata.name" :item="item"  ></NodeListItem>
  </template>
  <CardBlock v-if="list.isLoading.value" :content-loading="true"></CardBlock>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { useRoute } from "vue-router";

import useListDynamic from "@lib/nxn-common/composables/useListDynamic";

import Node from "@/models/Node";

import CardBlock from "@/components/common/card/CardBlock.vue";
import NodeListItem from "@/components/node/NodeListItem.vue";

const route = useRoute();

const filter = reactive({
  "node.deckhouse.io/group": route.params.ng_name,
});

const list = useListDynamic<Node>(
  Node,
  {
    sortBy: (a: Node, b: Node) => {
      return Date.parse(b.metadata.creationTimestamp) - Date.parse(a.metadata.creationTimestamp);
    },

    onLoadError: (error: any) => {
      console.error("NotImplementedError: ReleaseItemsList.onLoadError: " + JSON.stringify(error));
    },
  },
  filter,
  null,
  true
);

list.activate();
</script>

<template>
  <template v-if="!isLoading">
    <NodeListItem v-for="item in items" :key="item.primaryKey()" :item="item"></NodeListItem>
  </template>
  <CardBlock v-if="isLoading" :content-loading="true"></CardBlock>
  <CardEmpty v-if="!isLoading && items.length == 0" />
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";

import type Node from "@/models/Node";

import useLoadAll from "@/composables/useLoadAll";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardEmpty from "@/components/common/card/CardEmpty.vue";
import NodeListItem from "@/components/node/NodeListItem.vue";

const route = useRoute();
const { isLoading, lists } = useLoadAll();

// KOSTYL: filter by nodegroup
const items = computed<Node[]>(() =>
  lists.nodes.items.filter((node: Node) => !route.params.ng_name || node.nodeGroupName == route.params.ng_name.toString())
);
</script>

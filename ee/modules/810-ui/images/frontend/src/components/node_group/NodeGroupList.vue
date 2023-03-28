<template>
  <template v-if="!isLoading">
    <NodeGroupListItem v-for="item in lists.nodeGroups?.items" :key="item.metadata.name" :item="item"></NodeGroupListItem>
  </template>
  <CardBlock v-if="isLoading" :content-loading="true"></CardBlock>
  <CardEmpty v-if="!isLoading && lists.nodeGroups?.items.length == 0" />
</template>

<script setup lang="ts">
import useLoadAll from "@/composables/useLoadAll";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardEmpty from "@/components/common/card/CardEmpty.vue";
import NodeGroupListItem from "@/components/node_group/NodeGroupListItem.vue";

const { lists, isLoading } = useLoadAll();
</script>

<template>
  <template v-if="!isLoading">
    <InstanceClassesListItem v-for="item in lists.instanceClasses?.items" :key="item.primaryKey" :item="item"></InstanceClassesListItem>
  </template>
  <CardBlock v-if="isLoading" :content-loading="!!isLoading"></CardBlock>
  <CardEmpty v-if="!isLoading && lists.instanceClasses?.items.length == 0" />
</template>

<script setup lang="ts">
import CardBlock from "@/components/common/card/CardBlock.vue";
import CardEmpty from "@/components/common/card/CardEmpty.vue";
import InstanceClassesListItem from "@/components/instanceclass/InstanceClassesListItem.vue";
import useLoadAll from "@/composables/useLoadAll";

const { lists, isLoading } = useLoadAll();
</script>

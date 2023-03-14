<template>
  <div class="border-b border-gray-200">
    <nav class="flex space-x-2" aria-label="Tabs" role="tablist">
      <TabsItem v-for="item in items" :key="item.id" :item="item" />
    </nav>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from "vue";
import type { ITabsItem } from "@/types";
import TabsItem from "./TabsItem.vue";

const props = defineProps({
  items: Array as PropType<Array<ITabsItem>>,
});
</script>

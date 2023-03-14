<template>
  <router-link :to="{ name: item.routeName }" type="button" :class="getTabStyles(item)">
    {{ item.title }}
    <span v-if="item.badge" class="ml-1 py-0.5 px-1.5 rounded-full text-xs font-medium bg-blue-100 text-blue-500">{{ item.badge.value || '...' }}</span>
  </router-link>
</template>

<script setup lang="ts">
import type { PropType } from "vue";
import { computed } from "vue";
import type { ITabsItem } from "@/types";
import { useRoute } from "vue-router";

const route = useRoute();

const props = defineProps({
  item: {
    type: Object as PropType<ITabsItem>,
    required: true,
  },
});

const active = computed(() => route.name == props.item.routeName);

function getTabStyles(item: ITabsItem) {
  let styles =
    "py-4 px-4 inline-flex items-center gap-2 border-b-[3px] border-transparent text-sm whitespace-nowrap text-gray-500 hover:text-blue-600";

  if (active.value) {
    styles += " hs-tab-active:border-blue-600 hs-tab-active:text-blue-600 active";
  } else {
    styles += " ";
  }

  return styles;
}
</script>

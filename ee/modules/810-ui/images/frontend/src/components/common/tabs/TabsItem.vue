<template>
  <router-link
    v-if="!item.disabled"
    :to="{ name: item.routeName, params: item.routeParams || {} }"
    type="button"
    :class="getTabStyles(item)"
  >
    {{ item.title }}

    <BadgeItem v-if="item.badge" :title="item.badge.value == null ? '...' : item.badge.value" class="bg-blue-100 text-blue-500" />
  </router-link>
</template>

<script setup lang="ts">
import type { PropType } from "vue";
import { computed } from "vue";
import type { TabsItem } from "@/types";
import { useRoute } from "vue-router";

import BadgeItem from "@/components/common/badge/BadgeItem.vue";

const route = useRoute();

const props = defineProps({
  item: {
    type: Object as PropType<TabsItem>,
    required: true,
  },
});

const active = computed(() => route.name == props.item.routeName);

function getTabStyles(item: TabsItem) {
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

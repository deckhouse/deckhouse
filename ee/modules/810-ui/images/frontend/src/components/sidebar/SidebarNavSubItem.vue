<template>
  <li>
    <a :class="getButtonStyles(item)" href="javascript:;">
      {{ item.title }}
    </a>
  </li>
</template>

<script setup lang="ts">

import type { PropType } from "vue";
import type { ISidebarItem } from "@/types";
import IconOverview from "@/components/common/icon/IconOverview.vue";

const props = defineProps({
  item: {
    type: Object as PropType<ISidebarItem>,
    required: true,
  },
});

function getButtonStyles(item: ISidebarItem) {
  let styles = "flex items-center py-3 pl-16 pr-6 text-sm";

  if (item.active) {
    styles += " bg-blue-500 text-white";
  } else {
    styles += " text-slate-700 hover:bg-gray-100";
  }

  return styles;
}
</script>

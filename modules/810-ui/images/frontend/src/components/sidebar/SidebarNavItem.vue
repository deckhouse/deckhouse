<template>
  <li>
    <router-link :to="{ name: item.routeNames[0] }" :class="getButtonStyles(item)">
      <component v-if="item.icon" :is="Icons[item.icon]" />
      {{ item.title }}

      <span v-if="item.children" class="ml-auto" @click.prevent="is_open = !is_open">
        <component :is="Icons['IconChevronDown']" v-if="!is_open" />
        <component :is="Icons['IconChevronUp']" v-if="is_open" />
      </span>
    </router-link>
  </li>
  <template v-if="is_open">
    <SidebarNavSubItem v-for="item in item.children" :key="item.key" :item="item" />
  </template>
</template>

<script setup lang="ts">
import { ref } from "vue";
import type { PropType } from "vue";
import type { ISidebarItem } from "@/types";
import * as Icons from "../common/icon";
import SidebarNavSubItem from "./SidebarNavSubItem.vue";

const is_open = ref(false);

const props = defineProps({
  item: {
    type: Object as PropType<ISidebarItem>,
    required: true,
  },
});

function getButtonStyles(item: ISidebarItem) {
  let styles = "flex items-center gap-x-3.5 py-3 px-6 text-sm";

  if (item.active) {
    styles += " bg-slate-400 text-white";
  } else {
    styles += " text-slate-700 hover:bg-slate-100";
  }

  return styles;
}
</script>

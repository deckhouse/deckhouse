<template>
  <li>
    <router-link :to="{ name: item.routeNames[0], params: item.routeParams || {} }" :class="getButtonStyles()">
      <component v-if="item.icon" :is="Icons[item.icon as keyof typeof Icons]" />
      {{ item.title }}
      <BadgeItem v-if="item.badge" :title="item.badge" class="ml-auto" :class="getBadgeStyles()" />

      <span v-if="item.children" class="ml-auto" @click.prevent="isExpanded = !isExpanded">
        <component :is="Icons['IconChevronDown']" v-if="!isExpanded" />
        <component :is="Icons['IconChevronUp']" v-if="isExpanded" />
      </span>
    </router-link>
  </li>
  <template v-if="isExpanded">
    <SidebarNavItem v-for="(i, idx) in item.children" :key="idx" :item="i" :depth="depth + 1" />
  </template>
</template>

<script setup lang="ts">
import { ref, computed, toRaw } from "vue";
import type { PropType } from "vue";
import type { ISidebarItem } from "@/types";
import * as Icons from "../common/icon";

import BadgeItem from "@/components/common/badge/BadgeItem.vue";

import { useRoute } from "vue-router";

const route = useRoute();

const isExpanded = ref(true);

const active = computed(() => {
  if (!route.name) return false;

  return (
    props.item.routeNames.includes(route.name.toString()) ||
    props.item.children?.find((ch) => route.name && ch.routeNames.includes(route.name.toString()))
  );
});

const props = defineProps({
  item: {
    type: Object as PropType<ISidebarItem>,
    required: true,
  },
  depth: {
    type: Number,
    default: 0,
    required: false,
  },
});

function getButtonStyles() {
  let styles = "flex items-center text-sm";
  if (props.depth < 1) {
    styles += " gap-x-2 py-3 px-6";
  } else {
    styles += " gap-x-2 py-3 pl-12 pr-6";
  }

  if (active.value) {
    styles += props.depth < 1 ? " text-white" : " text-slate-700";
    styles += props.depth < 1 ? " bg-slate-500" : " bg-slate-200";
  } else {
    styles += " text-slate-700 hover:bg-slate-100";
  }

  return styles;
}

function getBadgeStyles() {
  if (active.value && props.depth < 1) {
    return "bg-white text-slate-500";
  } else {
    return "bg-slate-400 text-white";
  }
}
</script>

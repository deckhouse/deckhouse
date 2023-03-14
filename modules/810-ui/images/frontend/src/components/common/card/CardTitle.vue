<template>
  <div class="flex flex-wrap gap-3 items-center mb-6" v-if="title">
    <router-link :to="route" v-if="route" class="underline">
      <h4 class="text-2xl text-slate-800">
        {{ title }}
      </h4>
    </router-link>
    <h4 v-else class="text-2xl text-slate-800">
      {{ title }}
    </h4>
    <component :is="Icons[icon]" v-if="icon" />
    <span
      class="inline-flex items-center gap-1.5 py-1 px-3 rounded-full text-xs font-medium text-white uppercase"
      :class="getBadgeStyles(item.type)"
      v-for="item in badges"
      :key="item.id"
    >
      {{ item.title }}
    </span>
    <div v-if="tooltip" class="text-slate-400">
      <component :is="Icons['IconInfo']" v-tippy="tooltip" />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from "vue";
import type { RouteLocationRaw } from "vue-router";
import type { IBadge } from "@/types";
import * as Icons from "@/components/common/icon";

const props = defineProps({
  title: String,
  tooltip: String,
  badges: Array as PropType<Array<IBadge>>,
  route: Object as PropType<RouteLocationRaw>,
  icon: String,
});

function getBadgeStyles(type: string): string {
  const classes = {
    default: "bg-black",
    warning: "bg-orange-400",
    info: "bg-slate-400",
    success: "bg-green-500",
  };
  return type ? classes[type as keyof typeof classes] : classes["default"];
}
</script>

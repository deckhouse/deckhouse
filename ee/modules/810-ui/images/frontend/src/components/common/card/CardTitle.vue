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
    <template v-if="badges">
      <BadgeItem
        :title="item.title"
        class="text-white uppercase"
        :class="getBadgeStyles(item.type)"
        :loading="item.loading"
        size="spaced"
        v-for="(item, idx) in badges"
        :key="idx"
      />
    </template>
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
import BadgeItem from "@/components/common/badge/BadgeItem.vue";

const props = defineProps({
  title: String,
  tooltip: String,
  badges: Array as PropType<Array<IBadge>>,
  route: Object as PropType<RouteLocationRaw>,
  icon: String as PropType<keyof typeof Icons>,
});

function getBadgeStyles(type: string): string {
  const classes = {
    default: "bg-black",
    warning: "bg-orange-400",
    error: "bg-red-400",
    info: "bg-slate-400",
    success: "bg-green-500",
  };
  return type ? classes[type as keyof typeof classes] : classes["default"];
}
</script>

<template>
  <span class="inline-flex items-center gap-1.5 rounded-full text-xs font-medium" :class="size_styles[size].container">
    <component v-if="loading" :is="Icons['IconSpinner']" :class="size_styles[size].icon" />
    {{ title }}
  </span>
</template>

<script setup lang="ts">
import type { PropType } from "vue";
import * as Icons from "../icon";

const props = defineProps({
  title: [String, Number],
  loading: Boolean,
  size: {
    type: String as PropType<keyof typeof size_styles>,
    default: "default",
  },
});

const size_styles = {
  default: {
    container: "py-0 px-1.5",
    icon: "h-2 w-2",
  },
  spaced: {
    container: "py-1 px-2",
    icon: "h-3 w-3",
  },
};
</script>

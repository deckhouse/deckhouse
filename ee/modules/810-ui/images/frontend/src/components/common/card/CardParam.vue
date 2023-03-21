<template>
  <div class="flex flex-col">
    <div :class="[classes[type].wrapper, value || $slots.default ? '' : 'opacity-25']">
      <span :class="classes[type].title" class="block text-sm uppercase text-slate-800 mb-1">{{ title }}</span>
      <span v-if="typeof value === 'object'">
        <span :class="classes[type].text" v-for="val in value" :key="val">
          {{ val }}
        </span>
      </span>
      <span v-else :class="classes[type].text">
        <slot>
          {{ value || $slots.default ? value : '–' }}
        </slot>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps({
  title: String,
  value: {
    type: [String, Array, Number],
    required: false,
  },
  type: {
    type: String,
    default: "default",
  },
});

const classes = {
  default: {
    wrapper: "flex flex-col",
    title: "text-sm",
    text: "block text-sm text-slate-500",
  },
  row: {
    wrapper: "flex gap-1",
    title: "text-sm",
    text: "text-sm text-slate-500",
  },
  col_spaced: {
    wrapper: "flex flex-col",
    title: "text-sm mb-3",
    text: "block text-sm text-slate-500",
  },
} as any;
</script>

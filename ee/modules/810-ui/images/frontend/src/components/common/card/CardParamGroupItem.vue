<template>
  <div class="p-3 rounded-md border min-w-[120px] relative" :class="getStyles(state)">
    <span class="block text-sm text-slate-500 mb-1">{{ title }}</span>
    <div class="flex justify-between items-center">
      <span class="block text-2xl font-bold text-slate-500">
        {{ value }}
        <slot />
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import Button from "primevue/button";
import OverlayPanel from "primevue/overlaypanel";
import InputText from "primevue/inputtext";

import { ref, type PropType } from "vue";
import { number, string } from "zod";

const op = ref();
const toggle = (event: Event) => {
  op.value.toggle(event);
};

const props = defineProps({
  title: String,
  state: {
    type: String,
    required: false,
    default: "default",
  },
  value: {
    type: [String, Number] as PropType<string | number>,
    required: false,
  },
});

function getStyles(state: string | undefined) {
  const classes = {
    default: "",
    danger: "border-red-300 bg-red-50",
  };
  return state ? classes[state as keyof typeof classes] : classes["default"];
}
</script>

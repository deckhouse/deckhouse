<template>
  <div class="flex flex-col items-center justify-center flex-1">
    <h1 class="block text-[8rem] font-bold text-slate-600">{{ loadError.code }}</h1>
    <p class="text-slate-600 text-lg">{{ loadError.text }}</p>
  </div>
</template>

<script setup lang="ts">
import type { LoadError } from "@/types";
import type { PropType } from "vue";

const props = defineProps({
  loadError: {
    type: Object as PropType<LoadError>,
    required: true,
  },
});
</script>

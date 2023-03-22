<template>
  <div class="flex flex-col items-center justify-center flex-1">
    <h1 class="block text-[8rem] font-bold text-slate-600">{{ code }}</h1>
    <p class="text-slate-600 text-lg">{{ text }}</p>
  </div>
</template>

<script setup lang="ts">
const props = defineProps({
  code: String,
  text: String
});
</script>

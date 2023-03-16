<template>
  <span class="py-1 px-3 bg-slate-50 text-slate-500 border rounded-md text-xs">{{ title }}</span>
</template>

<script setup lang="ts">
const props = defineProps({
  title: String,
});
</script>

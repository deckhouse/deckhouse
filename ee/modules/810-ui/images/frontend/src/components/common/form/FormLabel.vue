<template>
  <label class="block font-medium text-gray-800">{{ value }}</label>
</template>

<script setup lang="ts">
const props = defineProps({
  value: String
});
</script>
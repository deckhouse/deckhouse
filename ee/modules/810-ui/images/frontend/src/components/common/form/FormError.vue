<template>
  <span class="block text-sm text-red-500">
    {{ text }}
    <slot />
  </span>
</template>

<script setup lang="ts">
const props = defineProps({
  text: String
});
</script>

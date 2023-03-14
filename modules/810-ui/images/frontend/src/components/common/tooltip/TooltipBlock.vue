<template>
  <div class="hs-tooltip inline-block">
    <div class="hs-tooltip-toggle">
      <slot />
      <span class="hs-tooltip-content hs-tooltip-shown:opacity-100 hs-tooltip-shown:visible opacity-0 transition-opacity inline-block absolute invisible z-10 py-1 px-2 bg-gray-900 text-xs font-medium text-white rounded-md shadow-sm" role="tooltip">
        {{ text }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps({
  text: String,
});
</script>
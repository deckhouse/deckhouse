<template>
    <div>
      <span class="block text-sm text-green-500" style="font-family: monospace;">{{ spec }}</span>
    </div>
</template>

<script setup lang="ts">
const props = defineProps({
  spec: String
});
</script>
<template>
  <div>
    <span class="block text-sm text-green-500 break-words" style="font-family: monospace" v-if="specMode">{{ spec }}</span>
  </div>
</template>

<script setup lang="ts">
import useSpecMode from "@/composables/useSpecMode";

const { specMode } = useSpecMode();

const props = defineProps({
  spec: String,
});
</script>

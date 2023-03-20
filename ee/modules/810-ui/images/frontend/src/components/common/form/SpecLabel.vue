<template>
    <div>
      <span class="block text-sm text-green-500" style="font-family: monospace;" v-if="globalSettings['specMode']">{{ spec }}</span>
    </div>
</template>

<script setup lang="ts">
import { inject } from "vue";
const globalSettings = inject("globalSettings");

const props = defineProps({
  spec: String
});
</script>
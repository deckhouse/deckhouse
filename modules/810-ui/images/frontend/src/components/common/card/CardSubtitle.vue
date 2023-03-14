<template>
  <div class="flex flex-wrap gap-3 items-center mb-3" v-if="title">
    <span class="block font-medium text-lg text-gray-800">
      {{ title }}
    </span>
    <div v-if="tooltip" class="text-gray-400">
      <component :is="Icons['IconInfo']" v-tippy="tooltip" />
    </div>
  </div>
</template>

<script setup lang="ts">
import * as Icons from "@/components/common/icon";

const props = defineProps({
  title: String,
  tooltip: String,
});

</script>

<template>
  <div>
    <CardSubtitle :title="title" />
    <div class="flex items-start space-x-6">
      <slot></slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import CardSubtitle from "@/components/common/card/CardSubtitle.vue";

const props = defineProps({
  title: String,
  title_value: {
    type: String,
    required: false,
  },
  value: {
    type: [String, Array],
    required: false,
  },
});
</script>

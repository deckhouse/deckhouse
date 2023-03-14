<template>
  <div>
    <CardSubtitle :title="title" :tooltip="tooltip" />
    <SpecLabel :spec="spec" v-if="spec" class="-mt-3 mb-3" />
  </div>
</template>

<script setup lang="ts">
import CardSubtitle from "@/components/common/card/CardSubtitle.vue";
import SpecLabel from "@/components/common/form/SpecLabel.vue";
const props = defineProps({
  title: String,
  tooltip: String,
  spec: String,
});
</script>

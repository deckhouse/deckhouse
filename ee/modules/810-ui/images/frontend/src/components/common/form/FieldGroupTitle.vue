<template>
  <div class="mb-3">
    <CardSubtitle :title="title" :tooltip="tooltip" />
    <SpecLabel :spec="spec" v-if="spec" class="-mt-6 mb-6" />
    <span class="block text-sm text-slate-400 -mt-6 mb-6 leading-normal w-[450px]" v-html="help" v-if="help" />
  </div>
</template>

<script setup lang="ts">
import CardSubtitle from "@/components/common/card/CardSubtitle.vue";
import SpecLabel from "@/components/common/form/SpecLabel.vue";
const props = defineProps({
  title: String,
  tooltip: String,
  spec: String,
  help: String
});
</script>

<template>
    <div class="flex justify-between items-start gap-2" :class="classes[type].container">
      <FieldLabel :title="title" :spec="spec" :tooltip="tooltip" :help="help" />
      <div class="flex items-start gap-2" :class="classes[type].input">
        <slot></slot>
        <Button icon="pi pi-replay" v-tippy="'Сбросить'" class="p-button-rounded p-button-primary p-button-sm p-button-text shrink-0" />
      </div>
    </div>
</template>

<script setup lang="ts">
import Button from "primevue/button";
import FieldLabel from "@/components/common/form/FieldLabel.vue";

const classes = {
  "default": {
    "container": "w-[450px]",
    "input": "ml-auto"
  },
  "wide": {
    "container": "w-[450px]",
    "input": "ml-auto"
  },
  "column": {
    "container": "w-[450px] flex-col",
    "input": "mt-1 w-full"
  },
}
const props = defineProps({
  title: String,
  help: String,
  spec: String,
  tooltip: String,
  type: {
    type: String,
    required: false,
    default: 'default'
  }
});
</script>
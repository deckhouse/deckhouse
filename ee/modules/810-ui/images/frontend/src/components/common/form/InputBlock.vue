<template>
    <div class="flex justify-between items-start gap-2" :class="getBlockClasses(type, special)">
      <div>
        <FieldLabel :title="title" :spec="spec" :tooltip="tooltip" :required="required" />
        <span class="block text-sm text-slate-400 mt-1 leading-tight" v-html="help" v-if="help && type != 'column'" />
      </div>
      <div class="flex items-center gap-2" :class="type_classes[type].input">
        <slot></slot>
        <Button icon="pi pi-replay" v-tippy="'Сбросить'" class="p-button-rounded p-button-primary p-button-sm p-button-text shrink-0" v-if="reset" />
        <InputSwitch v-if="toggle" />
      </div>
      <span class="block text-sm text-slate-400 mt-1 leading-tight" v-html="help" v-if="help && type == 'column'" />
    </div>
</template>

<script setup lang="ts">
import Button from "primevue/button";
import FieldLabel from "@/components/common/form/FieldLabel.vue";
import InputSwitch from 'primevue/inputswitch';

const type_classes = {
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

const special_classes = "bg-slate-50 p-6 rounded-md";

function getBlockClasses (type: string | undefined, special: string | undefined): string {
  const type_class = type_classes[type as keyof typeof type_classes].container;
  const special_class = special == true ? special_classes : "";
  return "".concat(type_class, " ", special_class)
}
const props = defineProps({
  title: String,
  help: String,
  spec: String,
  tooltip: String,
  reset: Boolean,
  required: Boolean,
  toggle: Boolean,
  special: Boolean,
  type: {
    type: String,
    required: false,
    default: 'default'
  }
});
</script>
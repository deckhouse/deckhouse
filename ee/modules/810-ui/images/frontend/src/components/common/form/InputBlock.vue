<template>
  <div class="flex justify-between items-start gap-2" :class="getBlockClasses(type, special)">
    <div class="min-w-0">
      <FieldLabel :title="title" :spec="spec" :tooltip="tooltip" :required="required" />
      <span class="block text-sm text-slate-400 mt-1 leading-tight" v-html="help" v-if="help && type != 'column'" />
      <FormError v-if="errorMessage && type != 'column'" :text="errorMessage" />
    </div>
    <div class="flex items-center gap-2" :class="type_classes[type].input">
      <slot></slot>
      <Button
        icon="pi pi-replay"
        v-tippy="'Сбросить'"
        class="p-button-rounded p-button-primary p-button-sm p-button-text shrink-0"
        v-if="reset && !disabled"
      />
      <InputSwitch v-if="toggle" :disabled="disabled" />
    </div>
    <FormError v-if="errorMessage && type == 'column'" :text="errorMessage" />
    <span class="block text-sm text-slate-400 mt-1 leading-tight" v-html="help" v-if="help && type == 'column'" />
  </div>
</template>

<script setup lang="ts">
import Button from "primevue/button";
import FieldLabel from "@/components/common/form/FieldLabel.vue";
import FormError from "@/components/common/form/FormError.vue";
import InputSwitch from "primevue/inputswitch";
import type { PropType } from "vue";

const type_classes = {
  default: {
    container: "w-[450px]",
    input: "ml-auto",
  },
  wide: {
    container: "w-[450px]",
    input: "ml-auto",
  },
  column: {
    container: "w-[450px] flex-col",
    input: "mt-1 w-full",
  },
};

const special_classes = "bg-slate-50 p-6 rounded-md";

function getBlockClasses(type: keyof typeof type_classes, special: boolean): string {
  const type_class = type_classes[type].container;
  const special_class = special == true ? special_classes : "";
  return "".concat(type_class, " ", special_class);
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
  disabled: Boolean,
  type: {
    type: String as PropType<keyof typeof type_classes>,
    required: false,
    default: "default",
  },
  errorMessage: String,
});
</script>

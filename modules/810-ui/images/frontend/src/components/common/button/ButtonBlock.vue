<template>
  <button
    type="button"
    :disabled="disabled"
    class="py-2 px-3 inline-flex justify-center items-center gap-2 rounded-md border-2 focus:outline-none focus:ring-2 focus:ring-gray-300 focus:ring-offset-2 transition-all text-sm font-semibold hover:bg-gray-800 hover:border-gray-800 hover:text-white"
    :class="getButtonStyles(type, disabled, loading)"
  >
    <component v-if="loading" :is="Icons['IconSpinner']" />
    <component v-if="icon" :is="Icons[icon]" />
    {{ title }}
  </button>
</template>

<script setup lang="ts">
import * as Icons from "../icon";
import type { PropType } from "vue";

const props = defineProps({
  title: String,
  type: String,
  disabled: Boolean,
  loading: Boolean,
  icon: String as PropType<keyof typeof Icons>,
});

function getButtonStyles(type: string | 'default', disabled: boolean | undefined, loading: boolean | undefined) {
  const types_classes = {
    default:            "border-gray-900 text-gray-800",
    "default-inverse":  "border-white text-white",
    subtle:             "border-dashed border-blue-300 text-blue-300",
    primary:            "bg-blue-500 border-blue-500 text-white",
    "primary-subtle":   "bg-white border-blue-500 text-blue-500",
    "primary-inverse":  "bg-white border-white text-blue-500",
    danger:             "bg-red-500 border-red-500 text-white",
    "danger-subtle":    "bg-white border-red-500 text-red-500",
  };

  const disabled_classes = "opacity-50 cursor-not-allowed";

  const type_class = type ? types_classes[type as keyof typeof types_classes] : types_classes["default"];
  const disabled_class = disabled == true || loading == true ? disabled_classes : "";

  return "".concat(type_class, " ", disabled_class)
}
</script>

<template>
  <div :class="getGridStyles(mode)"><slot /></div>
</template>

<script setup lang="ts">
const props = defineProps({
  mode: String,
});

function getGridStyles(mode: string | undefined) {
  const classes = {
    default: "flex flex-col gap-6",
    form: "grid grid-cols-2 gap-6",
  };
  return mode ? classes[mode as keyof typeof classes] : classes["default"];
}
</script>

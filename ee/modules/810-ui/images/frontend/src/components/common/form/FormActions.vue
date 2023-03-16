<template>
  <div class="h-40" />
  <div
    class="fixed bottom-4 pl-72 pr-6 inset-x-0 flex flex-wrap justify-start flex-nowrap z-50 w-full text-sm"
    :class="classes[compact == true ? 'compact' : 'default']"
  >
    <nav class="relative w-full bg-blue-500 rounded-md py-6 px-6 flex">
      <div class="flex items-center justify-between gap-6">
        <ButtonBlock title="Сохранить изменения" type="primary-inverse" :loading="submitLoading" @click="emit('submit')"></ButtonBlock>
        <ButtonBlock title="Отменить изменения" type="default-inverse" @click="emit('reset')"></ButtonBlock>
      </div>
    </nav>
  </div>
</template>

<script setup lang="ts">
import ButtonBlock from "../button/ButtonBlock.vue";

const emit = defineEmits(["submit", "reset"]);

const props = defineProps({
  compact: {
    type: Boolean,
    default: true,
  },
  submitLoading: {
    type: Boolean,
    default: false,
  },
});

const classes = {
  default: "",
  compact: "max-w-screen-2xl",
};
</script>

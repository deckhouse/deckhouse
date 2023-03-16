<template>
  <div class="p-4 rounded-md border min-w-[150px] relative" :class="getStyles(state)">
    <span class="block text-sm text-slate-500 mb-2">{{ title }}</span>
    <div class="flex justify-between items-center">
      <span class="block text-2xl font-bold text-slate-500">
        {{ value }}
      </span>
      <Button icon="pi pi-pencil" v-tippy="'Изменить'" v-if="edit == true" @click="toggle"
        class="p-button-primary p-button-sm p-button-raised p-button-rounded p-button-text
        absolute -bottom-2 -right-2" style="position: absolute" />
      <OverlayPanel ref="op">
        <div class="flex gap-x-3">
          <div>
            <label class="block text-sm font-medium text-gray-800 mb-2">Минимум:</label>
            <InputText class="w-[100px] p-inputtext-sm"/>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-800 mb-2">Максимум:</label>
            <InputText class="w-[100px] p-inputtext-sm"/>
          </div>
        </div>
      </OverlayPanel>  
    </div>
  </div>
</template>

<script setup lang="ts">
import Button from "primevue/button";
import OverlayPanel from 'primevue/overlaypanel';
import InputText from 'primevue/inputtext';

import FormLabel from "@/components/common/form/FormLabel.vue";

import { ref } from "vue";

const op = ref();
const toggle = (event) => {
    op.value.toggle(event);
}

const props = defineProps({
  title: String,
  edit: {
    type: Boolean,
    required: false,
    default: false
  },
  state: {
    type: String,
    required: false,
    default: 'default'
  },
  value: {
    type: String,
    required: false,
  },
});

function getStyles(state: string | undefined) {
  const classes = {
    default: "",
    danger: "border-red-300 bg-red-50",
  }
  return state ? classes[state as keyof typeof classes] : classes['default'];
}
</script>

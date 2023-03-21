<template>
  <div>
    <FieldGroupTitle :title="title" :spec="spec" />
    <table>
      <tr v-if="default">
        <td :colspan="fields.length" class="pb-3 pr-6">
          <div class="text-sm uppercase text-slate-400">
            {{ default }}
          </div>
        </td>
        <td></td>
      </tr>

      <tr>
        <td class="pb-3 pr-6" v-for="(field, idx) in fields" :key="idx">
          <div class="text-sm uppercase text-slate-500">
            {{ field }}
          </div>
        </td>
        <td></td>
      </tr>
      <tr v-for="(item, m_idx) in model" :key="m_idx">
        <td class="pb-3 pr-6" v-for="(field, f_idx) in fields" :key="f_idx">
          <InputText
            class="p-inputtext-sm w-[300px] lg:w-[200px] md:w-[150px]"
            :value="item.value[field]"
            :disabled="disabled"
            @change="handleChange(item, field, $event)"
          />
        </td>
        <td class="pb-3" v-if="!disabled">
          <Button
            @click="emit('remove', m_idx)"
            icon="pi pi-times"
            s
            class="p-button-rounded p-button-danger p-button-outlined p-button-sm"
          />
        </td>
      </tr>
      <tr>
        <td :colspan="fields.length + 1">
          <Button v-if="!disabled" @click="emit('push')" label="Добавить" class="p-button-outlined p-button-info w-full" />
        </td>
      </tr>
    </table>
  </div>
</template>

<script setup lang="ts">
import InputText from "primevue/inputtext";
import Button from "primevue/button";

import FieldGroupTitle from "@/components/common/form/FieldGroupTitle.vue";
import type { PropType } from "vue";

const props = defineProps({
  model: {
    type: Array as PropType<any[]>,
    required: true,
  },
  fields: {
    type: Array as PropType<string[]>,
    required: true,
  },
  title: String,
  disabled: Boolean,
  spec: String,
  default: String,
});

function handleChange(item: any, field: string, event: Event): void {
  const target = event.target as HTMLInputElement;

  item.value[field] = target.value;
  emit("change");
}

const emit = defineEmits(["push", "remove", "change"]);
</script>

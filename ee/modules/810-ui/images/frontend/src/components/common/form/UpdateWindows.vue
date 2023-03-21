<template>
  <div>
    <FieldArray :name="pathFor('windows')" v-slot="{ fields, push, remove }">
      <InputRow v-for="(window, index) in fields" :key="index" class="mb-6">
        <Field :name="pathFor(`windows[${index}].days`)" v-slot="{ errorMessage }">
          <MultiSelect
            v-model="localModelValue[index].days"
            :options="weekDaysOptions"
            :class="{ 'p-invalid': !!errorMessage }"
            optionLabel="name"
            optionValue="value"
            placeholder="Выберите дни"
            class="w-[275px]"
            :disabled="disabled"
            @change="onChange"
          />
          <InlineMessage v-if="errorMessage">{{ errorMessage }}</InlineMessage>
        </Field>
        <Field :name="pathFor(`windows[${index}].from`)" v-slot="{ errorMessage }">
          <FormLabel value="С" />
          <Calendar
            v-model="localModelValue[index].from"
            :class="{ 'p-invalid': !!errorMessage }"
            :showTime="true"
            :timeOnly="true"
            class="w-[75px]"
            :disabled="disabled"
            @update:modelValue="onChange"
          />
          <InlineMessage v-if="errorMessage">{{ errorMessage }}</InlineMessage>
        </Field>
        <Field :name="pathFor(`windows[${index}].to`)" v-slot="{ errorMessage }">
          <FormLabel value="До" />
          <Calendar
            v-model="localModelValue[index].to"
            :class="{ 'p-invalid': !!errorMessage }"
            :showTime="true"
            :timeOnly="true"
            class="w-[75px]"
            :disabled="disabled"
            @update:modelValue="onChange"
          />
          <InlineMessage v-if="errorMessage">{{ errorMessage }}</InlineMessage>
        </Field>
        <Button icon="pi pi-times" class="p-button-rounded p-button-danger p-button-outlined" @click="remove(index)" v-if="!disabled" />
      </InputRow>
      <Button
        label="Добавить"
        class="p-button-outlined p-button-info w-[625px]"
        @click="push({ days: [], from: '00:00', to: '03:00' })"
        v-if="!disabled"
      />
    </FieldArray>
  </div>
</template>

<script setup lang="ts">
import { computed, type PropType } from "vue";
import { Field, FieldArray } from "vee-validate";
import dayjs from "dayjs";
import type { IUpdateWindow } from "@/types";

import Button from "primevue/button";
import InputRow from "./InputRow.vue";
import Calendar from "primevue/calendar";
import FormLabel from "./FormLabel.vue";
import MultiSelect from "primevue/multiselect";
import InlineMessage from "primevue/inlinemessage";

const props = defineProps({
  modelValue: {
    type: Object as PropType<IUpdateWindow[]>,
  },
  disabled: Boolean,
  fieldNamePath: {
    type: String,
    default: "",
  },
});

const localModelValue = computed(() => {
  let windows = props.modelValue || [];
  return windows.map((window: IUpdateWindow): any => ({
    days: window.days,
    from: stringToDate(window.from),
    to: stringToDate(window.to),
  }));
});

const weekDaysOptions = [
  { name: "Пн", value: "Mon" },
  { name: "Вт", value: "Tue" },
  { name: "Ср", value: "Wed" },
  { name: "Чт", value: "Thu" },
  { name: "Пт", value: "Fri" },
  { name: "Сб", value: "Sat" },
  { name: "Вс", value: "Sun" },
];

function pathFor(attribute: string): string {
  if (!props.fieldNamePath.length) return attribute;
  else return [props.fieldNamePath, attribute].join(".");
}

const timeFormat = "HH:mm";

function stringToDate(str: string): Date {
  return dayjs(str, timeFormat).toDate();
}

function dateToString(date: Date): string {
  return dayjs(date).format(timeFormat);
}

function onChange() {
  emit(
    "update:modelValue",
    localModelValue.value.map(
      (window: any): IUpdateWindow => ({
        days: window.days,
        from: dateToString(window.from),
        to: dateToString(window.to),
      })
    )
  );
}

const emit = defineEmits(["update:modelValue"]);
</script>

<template>
  <div class="flex flex-wrap items-start gap-x-24 gap-y-6 mb-12">
    <FieldArray :name="pathFor('labelsAsArray')" v-slot="{ push, remove, fields }">
      <InputLabelGroup
        :class="inputClass"
        title="Лейблы"
        :fields="['key', 'value']"
        :disabled="disabled"
        :model="fields"
        @remove="remove"
        @push="push({ key: undefined, value: undefined })"
      />
    </FieldArray>
    <FieldArray :name="pathFor('annotationsAsArray')" v-slot="{ push, remove, fields }">
      <InputLabelGroup
        :class="inputClass"
        title="Аннотации"
        :fields="['key', 'value']"
        :disabled="disabled"
        :model="fields"
        @remove="remove"
        @push="push({ key: undefined, value: undefined })"
      />
    </FieldArray>
    <FieldArray :name="pathFor('taints')" v-slot="{ push, remove, fields }">
      <InputLabelGroup
        :class="inputClass"
        title="Тейнты"
        :fields="['effect', 'key', 'value']"
        :disabled="disabled"
        :model="fields"
        @remove="remove"
        @push="push({ key: undefined, value: undefined, effect: undefined })"
      />
    </FieldArray>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from "vue";
import { FieldArray } from "vee-validate";
import InputLabelGroup from "@/components/common/form/InputLabelGroup.vue";
import type { IKeyValue, ITaint } from "@/types";

type NodeTemplate = {
  labelsAsArray: IKeyValue[];
  annotationsAsArray: IKeyValue[];
  taints: ITaint[];
};

const props = defineProps({
  modelValue: {
    type: Object as PropType<NodeTemplate>,
    required: true,
  },
  inputClass: String,
  disabled: Boolean,
  namePath: {
    type: String,
    default: "nodeTemplate",
  },
});

function pathFor(attribute: "labelsAsArray" | "annotationsAsArray" | "taints"): string {
  if (!props.namePath.length) return attribute;
  else return [props.namePath, attribute].join(".");
}

const emit = defineEmits(["update:modelValue"]);
</script>

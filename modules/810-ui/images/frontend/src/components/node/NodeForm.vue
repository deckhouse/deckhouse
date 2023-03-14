<template>
  <GridBlock>
    <CardBlock title="Состояние узла" notice-placement="top" notice-type="warning" :badges="badges">
      <template #content>
        <CardTable :fields="table_fields" :data="table_data" />

        <CardDivider />

        <CardTitle title="Шаблон узла" />
        <div class="flex flex-wrap items-start -mx-12 -my-6">
          <FieldArray :name="'labelsAsArray'" v-slot="{ push, remove, fields }" v-model="values.labelsAsArray">
            <InputLabelGroup
              class="mx-12 my-6"
              title="Лейблы"
              :fields="['key', 'value']"
              :disabled="readonly"
              :model="fields"
              @remove="remove"
              @push="push({ key: '', value: '' })"
            />
          </FieldArray>
          <FieldArray :name="'annotationsAsArray'" v-slot="{ push, remove, fields }" v-model="values.annotationsAsArray">
            <InputLabelGroup
              class="mx-12 my-6"
              title="Аннотации"
              :fields="['key', 'value']"
              :disabled="readonly"
              :model="fields"
              @remove="remove"
              @push="push({ key: '', value: '' })"
            />
          </FieldArray>
          <FieldArray :name="'taints'" v-slot="{ push, remove, fields }">
            <InputLabelGroup
              class="mx-12 my-6"
              title="Тейнты"
              :fields="['effect', 'key', 'value']"
              :disabled="readonly"
              :model="fields"
              @remove="remove"
              @push="push({ key: '', value: '', effect: '' })"
            />
          </FieldArray>
        </div>

        <CardDivider />

        <div class="flex flex-col gap-y-6">
          <div>
            <CardSubtitle title="Ресурсы" />
            <CardParamGrid>
              <CardParam title="CPU" value="16" />
              <CardParam title="Память" :value="formatBytes(node.status.allocatable.memory)" />
              <CardParam title="Эфемерное хранилище" :value="formatBytes(node.status.allocatable['ephemeral-storage'])" />
              <CardParam title="Количество подов" :value="node.status.allocatable.pods" />
            </CardParamGrid>
          </div>
          <div>
            <CardSubtitle title="Адреса" />
            <CardParamGrid>
              <CardParam title="Internal IP" :value="node.internalIP" />
              <CardParam title="External IP" :value="node.externalIP" />
              <CardParam title="Имя хоста" :value="node.hostname" />
              <CardParam title="Подсети для подов" type="col_spaced" :value="node.podCIDRs" />
            </CardParamGrid>
          </div>
          <div>
            <CardSubtitle title="Всякое" />
            <CardParamGrid>
              <CardParam title="Операционная система" :value="node.os" />
              <CardParam title="Архитектура" :value="node.arch" />
              <CardParam title="OS Image" :value="node.osImage" />
              <CardParam title="Container runtime" :value="node.cri" />
              <CardParam title="Версия Kubelet" :value="node.kubeletVersion" />
              <CardParam title="Версия Kubeproxy" :value="node.kubeproxyVersion" />
            </CardParamGrid>
          </div>
          <div>
            <CardParamGrid>
              <CardParam title="Machine ID" :value="node.machineID" class="col-span-2" />
              <CardParam title="System UUID" :value="node.systemUUID" class="col-span-2" />
              <CardParam title="Boot ID" :value="node.bootID" class="col-span-2" />
            </CardParamGrid>
          </div>
        </div>
      </template>
      <template #actions>
        <ButtonBlock
          title="Одобрить обновление с перезагрузкой"
          type="primary"
          v-if="node.needDisruptionApproval"
          @click="disruptionApprove"
        ></ButtonBlock>
        <ButtonBlock :title="node.unschedulable ? 'Uncordon' : 'Cordon'" type="primary-subtle" @click="toggleCordon"></ButtonBlock>
        <ButtonBlock title="Drain" type="primary-subtle" @click="drain" :loading="drainLoading"></ButtonBlock>
      </template>
      <template #notice v-if="node.errorMessage"> {{ node.errorMessage }} </template>
    </CardBlock>
    <FormActions :compact="false" v-if="!readonly && meta.dirty" @submit="submitForm" @reset="resetForm" />
  </GridBlock>
</template>

<script setup lang="ts">
import { computed, ref, } from "vue";
import type { PropType } from "vue";
import { useRouter } from "vue-router";

import { formatTime, formatBytes, objectAsArray, arrayToObject } from "@/utils";

import { useForm, FieldArray } from "vee-validate";
import { toFormValidator } from "@vee-validate/zod";
import { z } from "zod";

import type Node from "@/models/Node";

import GridBlock from "@/components/common/grid/GridBlock.vue";
import ButtonBlock from "@/components/common/button/ButtonBlock.vue";

import FormActions from "@/components/common/form/FormActions.vue";
import InputLabelGroup from "@/components/common/form/InputLabelGroup.vue";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardTitle from "@/components/common/card/CardTitle.vue";
import CardSubtitle from "@/components/common/card/CardSubtitle.vue";
import CardDivider from "@/components/common/card/CardDivider.vue";
import CardParam from "@/components/common/card/CardParam.vue";
import CardParamGrid from "@/components/common/card/CardParamGrid.vue";
import CardTable from "@/components/common/card/CardTable.vue";
import type { IBadge } from "@/types";

const router = useRouter();

const props = defineProps({
  node: {
    type: Object as PropType<Node>,
    required: true,
  },
  readonly: {
    type: Boolean,
    default: false,
  },
});

const drainLoading = ref(false);

const badges = computed<IBadge[]>(() => {
  return props.node
    ? [
        { id: 1, title: props.node.state, type: props.node.errorMessage ? "warning" : "success" },
        // { id: 2, title: "Scheduling", type: "warning" },
      ]
    : [];
});

const table_fields = ["Created at", "Type", "Reason", "Message"];

const table_data = computed(() => {
  return props.node.status.conditions.map((a: any) => {
    return {
      created_at: formatTime(a.lastTransitionTime),
      type: a.type,
      reason: a.reason,
      message: a.message,
    };
  });
});

// Validations Schema
const formSchema = z.object({
  // labels: z.record(z.string(), z.any()),
  // annotations: z.record(z.string(), z.any()),
  labelsAsArray: z
    .object({
      key: z.string().min(1),
      value: z.string().optional(),
    })
    .array(),
  annotationsAsArray: z
    .object({
      key: z.string().min(1),
      value: z.string().optional(),
    })
    .array(),
  taints: z
    .object({
      key: z.string().min(1),
      value: z.string().optional(),
      effect: z.string(),
    })
    .array()
    .optional(),
});

// Form

const initialValues = computed(() => {
  return {
    labelsAsArray: objectAsArray(props.node.metadata.labels),
    annotationsAsArray: objectAsArray(props.node.metadata.annotations),
    taints: props.node.spec.taints,
  };
});

const { handleSubmit, values, meta, resetForm, errors } = useForm({
  validationSchema: toFormValidator(formSchema),
  initialValues,
});

// Functions

const submitForm = handleSubmit(
  (values) => {
    console.log(JSON.stringify(values, null, 2));
    console.log(meta.value);

    // TODO: no prop mutation?
    // eslint-disable-next-line vue/no-mutating-props
    props.node.metadata.labels = arrayToObject(values.labelsAsArray);
    // eslint-disable-next-line vue/no-mutating-props
    props.node.metadata.annotations = arrayToObject(values.annotationsAsArray);
    // eslint-disable-next-line vue/no-mutating-props
    props.node.spec.taints = values.taints;

    props.node.save().then((a: any) => {
      console.log("Save response", a);
    });

    router.push({ name: "NodeShow" });
  },
  (err) => {
    console.error("Validation error", err, errors);
  }
);

function toggleCordon(): void {
  // TODO: fix unexpected mutation
  // eslint-disable-next-line vue/no-mutating-props
  props.node.spec.unschedulable = !props.node.spec.unschedulable;
  props.node.save().then((a: any) => {
    console.log("Save response", a);
  });
}

function drain() {
  drainLoading.value = true;
  props.node.drain().then((a: any) => {
    console.log("Drain response", a);
    drainLoading.value = false;
  });
}

function disruptionApprove() {
  props.node.disruptionApprove().then((a: any) => {
    console.log("disruptionApprove response", a);
    console.log(props.node.metadata.annotations, props.needDisruptionApproval);

  });
}
</script>

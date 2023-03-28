<template>
  <GridBlock>
    <CardBlock title="Состояние узла" notice-placement="top" notice-type="warning" :badges="badges">
      <template #content>
        <CardTable :fields="table_fields" :data="table_data" />

        <CardDivider />

        <CardTitle title="Шаблон узла" />
        <div class="flex flex-wrap items-start -mx-12 -my-6">
          <NodeTemplateInputs v-model="values" input-class="mx-12 my-6" :disabled="readonly" name-path=""> </NodeTemplateInputs>
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
        <NodeActions :node="node" />
      </template>
      <template #notice v-if="node.errorMessage"> {{ node.errorMessage }} </template>
    </CardBlock>
    <FormActions
      :compact="false"
      v-if="!readonly && (meta.dirty || submitLoading)"
      @submit="submitForm"
      @reset="resetForm"
      :submit-loading="submitLoading"
    />
  </GridBlock>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import type { PropType } from "vue";
import { onBeforeRouteLeave, useRouter } from "vue-router";

import { formatTime, formatBytes, objectAsArray, arrayToObject } from "@/utils";

import { useForm } from "vee-validate";
import { toFormValidator } from "@vee-validate/zod";
import { z } from "zod";
import useFormLeaveGuard from "@/composables/useFormLeaveGuard";

import type Node from "@/models/Node";

import GridBlock from "@/components/common/grid/GridBlock.vue";

import FormActions from "@/components/common/form/FormActions.vue";
import NodeTemplateInputs from "@/components/common/form/NodeTemplateInputs.vue";
import NodeActions from "@/components/node/NodeActions.vue";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardTitle from "@/components/common/card/CardTitle.vue";
import CardSubtitle from "@/components/common/card/CardSubtitle.vue";
import CardDivider from "@/components/common/card/CardDivider.vue";
import CardParam from "@/components/common/card/CardParam.vue";
import CardParamGrid from "@/components/common/card/CardParamGrid.vue";
import CardTable from "@/components/common/card/CardTable.vue";
import type { Badge } from "@/types";
import { nodeTemplateSchema } from "@/validations";

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

const submitLoading = ref(false);

const badges = computed<Badge[]>(() => {
  return props.node
    ? [
        { title: props.node.state, type: props.node.errorMessage ? "warning" : "success" },
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
const formSchema = nodeTemplateSchema;

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

if (!props.readonly) useFormLeaveGuard({ formMeta: meta, onLeave: resetForm });

// Functions

const submitForm = handleSubmit(
  (values) => {
    submitLoading.value = true;
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
      submitLoading.value = false;

      router.push({ name: "NodeShow" });
    });
  },
  (err) => {
    console.error("Validation error", err, errors);
  }
);
</script>

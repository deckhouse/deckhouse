<template>
  <ButtonBlock
    title="Одобрить обновление с перезагрузкой"
    type="primary"
    v-if="node.needDisruptionApproval"
    @click="disruptionApprove"
    :loading="disruptionApproveLoading"
  ></ButtonBlock>
  <ButtonBlock
    :title="node.unschedulable ? 'Uncordon' : 'Cordon'"
    type="primary-subtle"
    @click="toggleCordon"
    :loading="cordonLoading"
  ></ButtonBlock>
  <ButtonBlock title="Drain" type="primary-subtle" @click="drain" :loading="drainLoading"></ButtonBlock>
</template>

<script setup lang="ts">
import { ref, type PropType } from "vue";

import type Node from "@/models/Node";

import ButtonBlock from "@/components/common/button/ButtonBlock.vue";

const props = defineProps({
  node: {
    type: Object as PropType<Node>,
    required: true,
  },
});

const disruptionApproveLoading = ref(false);
const drainLoading = ref(false);
const cordonLoading = ref(false);

function toggleCordon(): void {
  cordonLoading.value = true;

  // TODO: fix unexpected mutation
  // eslint-disable-next-line vue/no-mutating-props
  props.node.spec.unschedulable = !props.node.spec.unschedulable;
  props.node.save().then((a: any) => {
    console.log("Save response", a);
    cordonLoading.value = false;
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
  disruptionApproveLoading.value = true;
  props.node.disruptionApprove().then((a: any) => {
    console.log("disruptionApprove response", a);
    console.log(props.node.metadata.annotations, props.node.needDisruptionApproval);
    disruptionApproveLoading.value = false;
  });
}
</script>

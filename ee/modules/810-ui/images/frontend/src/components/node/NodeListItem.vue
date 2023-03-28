<template>
  <CardBlock
    :title="item.metadata.name"
    :route="{ name: 'NodeShow', params: { ng_name: item.group, name: item.metadata.name } }"
    notice-type="warning"
    :badges="item.badges"
  >
    <template #content>
      <CardParamGrid>
        <CardParam title="Дата создания" :value="formatTime(item.metadata.creationTimestamp, 'DD MMM YYYY | HH:mm Z')" />
        <CardParam title="Зона" :value="item.zone" />
        <CardParam title="Internal IP" :value="item.internalIP" />
        <CardParam title="External IP" :value="item.externalIP" />
        <CardParam title="Версия kubelet" :value="item.kubeletVersion" />
        <CardParam title="CRI" :value="item.cri" />
        <CardParam title="Версия kernel" :value="item.kernelVersion" />
        <CardParam title="OS Image" :value="item.osImage" />
      </CardParamGrid>
    </template>
    <template #actions>
      <NodeActions :node="item" />
    </template>
    <template #notice v-if="item.errorMessage">
      {{ item.errorMessage }}
    </template>
  </CardBlock>
</template>

<script setup lang="ts">
import { formatTime } from "@/utils";

import type Node from "@/models/Node";

import CardBlock from "../common/card/CardBlock.vue";
import CardParamGrid from "../common/card/CardParamGrid.vue";
import CardParam from "../common/card/CardParam.vue";
import NodeActions from "./NodeActions.vue";

const props = defineProps({
  item: {
    type: Object as () => Node,
    required: true,
  },
});
</script>

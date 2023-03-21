<template>
  <CardBlock
    :title="item.metadata.name"
    :route="{ name: 'NodeGroupShow', params: { name: item.metadata.name } }"
    notice-type="danger"
    :badges="item.badges"
  >
    <template #content>
      <div class="flex flex-wrap items-start gap-x-12 gap-y-6 mb-6">
        <CardParam title="Тип узлов" :value="item.spec.nodeType" />
        <CardParam title="Приоритет группы" :value="item.priority" />
        <CardParam title="Версия Kubernetes" :value="item.kubernetesVersion" />
      </div>

      <div class="flex flex-wrap items-start gap-x-12 gap-y-6 mb-6">
        <CardParamGroup title="Состояние узлов">
          <CardParamGroupItem title="Всего узлов" :value="item.status.nodes" />
          <CardParamGroupItem title="Готовые" :value="item.status.ready" />
          <CardParamGroupItem title="Актуальные" :value="item.status.upToDate" />
        </CardParamGroup>

        <CardParamGroup title="Параметры автомасштабирования" v-if="item.isAutoscalable">
          <CardParamGroupItem title="Узлов на зону" :value="`${item.status.min || '?'}-${item.status.max || '?'}`" />
          <CardParamGroupItem title="Необходимо" :value="item.status.desired || '—'" />
          <CardParamGroupItem title="Заказано" :value="item.status.instances || '—'" />
          <CardParamGroupItem title="Резерв" :value="item.status.standby || '—'" />
        </CardParamGroup>

        <CardParam title="Зоны" :value="item.zones" v-if="item.isAutoscalable" />
        <CardParam title="Класс машин" :value="item.cloudInstanceKind" v-if="item.isAutoscalable" />
      </div>

      <div class="flex items-start justify-between gap-x-3 mt-6">
        <div class="flex flex-wrap items-start gap-x-3 gap-y-2">
          <CardLabel v-for="(key, value) of item.metadata.labels" :key="key" :title="`${key}: ${value}`" />
        </div>
        <div v-if="item.spec.nodeTemplate?.taints?.length">
          <span class="inline-flex py-1 px-2 rounded-full text-xs font-medium text-white bg-slate-500">
            Teйнты: {{ item.spec.nodeTemplate.taints.length }}
          </span>
        </div>
      </div>
    </template>
    <template #actions>
      <NodeGroupActions :item="item" />
    </template>
    <template #notice v-if="item.errorMessages.length">
      {{ item.errorMessages.join(";") }}
    </template>
  </CardBlock>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";

import CardBlock from "../common/card/CardBlock.vue";
import CardParam from "../common/card/CardParam.vue";
import CardParamGroup from "../common/card/CardParamGroup.vue";
import CardParamGroupItem from "../common/card/CardParamGroupItem.vue";
import CardLabel from "../common/card/CardLabel.vue";
import NodeGroupActions from "./NodeGroupActions.vue";

import type NodeGroup from "@/models/NodeGroup";

const props = defineProps({
  item: {
    type: Object as () => NodeGroup,
    required: true,
  },
});
</script>

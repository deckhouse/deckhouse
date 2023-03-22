<template>
  <CardParamGrid>
    <CardParam title="Тип" :value="item.spec?.machineType" />
    <CardParam title="Диск" :value="diskInfo" />
    <CardParam title="Доп сетевые теги" :value="item.spec?.additionalNetworkTags?.join(', ')" />
  </CardParamGrid>
  <CardLabel v-for="(key, value) of props.item.spec?.additionalLabels" :key="key" :title="`${key}: ${value}`" />
</template>

<script setup lang="ts">
import type InstanceClassBase from "@/models/instanceclasses/InstanceClassBase";
import { formatBytes } from "@/utils";
import { computed, type PropType } from "vue";

import type { IconsType } from "@/types";
import CardParam from "../../common/card/CardParam.vue";
import CardParamGrid from "../../common/card/CardParamGrid.vue";
import CardLabel from "../../common/card/CardLabel.vue";

const props = defineProps({
  item: {
    type: Object as PropType<InstanceClassBase>,
    required: true,
  },
});

const diskInfo = computed((): string | undefined => {
  return props.item.spec?.diskSizeGb ? `${props.item.spec?.diskSizeGb} ${props.item.spec?.diskType || "–"}` : undefined;
});
</script>

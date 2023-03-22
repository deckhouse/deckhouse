<template>
  <CardParamGrid>
    <CardParam title="ЦПУ" :value="item.spec?.numCPUs" />
    <CardParam title="Память" :value="memoryInfo" />
    <CardParam title="Диск" :value="diskInfo" />
    <CardParam title="Пул ресурсов" :value="item.spec?.resourcePool" />
    <CardParam title="Datastore" :value="item.spec?.datastore" />
    <CardParam title="Шаблон" :value="item.spec?.template" />
  </CardParamGrid>
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
  return props.item.spec?.rootDiskSize ? `${props.item.spec?.rootDiskSize} G` : undefined;
});

const memoryInfo = computed((): string | undefined => {
  return props.item.spec?.memory ? `${props.item.spec?.memory} M` : undefined;
});
</script>

<template>
  <CardBlock
    :title="item.name"
    :route="{ name: 'InstanceClassShow', params: { name: item.name } }"
    notice-placement="top"
    :badges="item.badges"
    :icon="icon"
  >
    <template #content>
      <CardParamGrid>
        <CardParam title="Тип VM" :value="item.spec?.instanceType" />
        <CardParam title="ЦПУ" :value="item.instanceTypeInfo?.VCPU" />
        <CardParam title="Память" :value="item.instanceTypeInfo?.MemoryMb ? `${item.instanceTypeInfo?.MemoryMb} MB` : undefined" />
        <CardParam title="Диск" :value="diskInfo" />
        <CardParam title="Доп группы безопасности" :value="item.spec?.additionalSecurityGroups?.join(', ')" />
      </CardParamGrid>
      <CardLabel v-for="(key, value) of props.item.spec?.additionalTags" :key="key" :title="`${key}: ${value}`" />
    </template>
    <template #actions>
      <InstanceClassActionsVue :item="item" />
    </template>
    <template #notice> TODO: Используется в 3 группах узлов: <b>big-node-group, redis, oopyachka-node</b> </template>
  </CardBlock>
</template>

<script setup lang="ts">
import type InstanceClassBase from "@/models/instanceclasses/InstanceClassBase";
import { formatBytes } from "@/utils";
import { computed, type PropType } from "vue";

import type { IconsType } from "@/types";
import CardBlock from "../common/card/CardBlock.vue";
import CardParam from "../common/card/CardParam.vue";
import CardParamGrid from "../common/card/CardParamGrid.vue";
import InstanceClassActionsVue from "./InstanceClassActions.vue";
import CardLabel from "../common/card/CardLabel.vue";

const props = defineProps({
  item: {
    type: Object as PropType<InstanceClassBase>,
    required: true,
  },
});

const icon = computed<IconsType | undefined>(() => {
  switch (props.item.constructor.klassName) {
    case "AwsInstanceClass": {
      return "IconAWSLogo";
    }
    case "OpenstackInstanceClass": {
      return "IconOpenStackLogo";
    }
    default: {
      return undefined;
    }
  }
});

const diskInfo = computed((): string | undefined => {
  switch (props.item.constructor.klassName) {
    case "AwsInstanceClass": {
      return `${props.item.spec?.diskSizeGb} G ${props.item.spec?.diskType}`;
    }
    case "OpenstackInstanceClass": {
      return props.item.spec?.rootDiskSizeGb ? `${props.item.spec?.rootDiskSizeGb} G` : undefined;
    }
    default: {
      return undefined;
    }
  }
});

const additionalTags = computed(() => {
  if (!props.item.spec?.additionalTags) return [];
  return Object.keys(props.item.spec?.additionalTags).map((key: string) => `${key}: ${props.item.spec?.additionalTags[key]}`);
});
</script>

<template>
  <CardBlock
    :title="item.name"
    :route="{ name: 'InstanceClassShow', params: { name: item.name } }"
    notice-placement="top"
    :badges="item.badges"
    :icon="icon"
  >
    <template #content>
      <component :is="itemParams" :item="item" />
    </template>
    <template #actions>
      <InstanceClassActionsVue :item="item" />
    </template>
    <template #notice v-if="!!nodeGroupConsumers?.length"> Используется в {{nodeGroupConsumers?.length}} группах узлов: <b>{{nodeGroupConsumers?.join(', ')}}</b> </template>
  </CardBlock>
</template>

<script setup lang="ts">
import type InstanceClassBase from "@/models/instanceclasses/InstanceClassBase";
import { formatBytes } from "@/utils";
import { computed, shallowRef, type PropType } from "vue";
import Discovery from "@/models/Discovery";

import type { IconsType } from "@/types";
import CardBlock from "../common/card/CardBlock.vue";
import InstanceClassActionsVue from "./InstanceClassActions.vue";
import params from "@/components/instanceclass/params"; // different params for different types

const props = defineProps({
  item: {
    type: Object as PropType<InstanceClassBase>,
    required: true,
  },
});

// TODO: uses hash const
const icon = computed<IconsType | undefined>(() => {
  switch (props.item.constructor.klassName) {
    case "AwsInstanceClass":        return "IconAWSLogo";
    case "AzureInstanceClass":      return "IconAzureLogo";
    case "GcpInstanceClass":        return "IconGCPLogo";
    case "OpenstackInstanceClass":  return "IconOpenStackLogo";
    case "VsphereInstanceClass":    return "IconVmWareLogo";
    case "YandexInstanceClass":     return "IconYandexCloudLogo";
    default:                        return undefined;
  }
});

const nodeGroupConsumers = computed((): string [] | undefined => {
  return props.item.status?.nodeGroupConsumers;
});

const discovery = Discovery.get();
const itemParams = shallowRef();
itemParams.value = params[discovery.cloudProvider.name as keyof typeof params];

</script>

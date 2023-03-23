<template>
  <template v-if="!list.isLoading.value">
    <InstanceClassesListItem v-for="item in list.items" :key="item.metadata.uid" :item="item"></InstanceClassesListItem>
  </template>
  <CardBlock v-if="list.isLoading.value" :content-loading="!!list.isLoading.value"></CardBlock>
  <CardEmpty v-if="!list.isLoading.value && list.items.length == 0" />
</template>

<script setup lang="ts">
import { watch, type PropType } from "vue";
import { useRoute } from "vue-router";

import useListDynamic from "@lib/nxn-common/composables/useListDynamic";

import type { InstanceClassesTypes } from "@/models/instanceclasses";
import Discovery from "@/models/Discovery";

import CardBlock from "@/components/common/card/CardBlock.vue";
import CardEmpty from "@/components/common/card/CardEmpty.vue";
import InstanceClassesListItem from "@/components/instanceclass/InstanceClassesListItem.vue";

const route = useRoute();

const discovery = Discovery.get();

const props = defineProps({
  sortBy: {
    type: String,
    required: true,
  },
});

watch(
  () => props.sortBy,
  () => list.resort()
);

const list = useListDynamic<InstanceClassesTypes>(
  discovery.instanceClassKlass,
  {
    sortBy: (a: InstanceClassesTypes, b: InstanceClassesTypes) => {
      switch (props.sortBy) {
        case "name": {
          return String(a.name).localeCompare(String(b.name));
        }
        default: {
          return Date.parse(b.creationTimestamp.toString()) - Date.parse(a.creationTimestamp.toString());
        }
      }
    },

    onLoadError: (error: any) => {
      console.error("NotImplementedError: ReleaseItemsList.onLoadError: " + JSON.stringify(error));
    },
  },
  {},
  null,
  true
);

list.activate();
</script>

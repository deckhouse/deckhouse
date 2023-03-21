<template>
  <PageTitle>{{ route.params.ng_name || "Узлы всех групп" }}</PageTitle>
  <PageActions>
    <template #tabs v-if="tabs">
      <TabsBlock :items="tabs" />
    </template>
    <template #filter>
      <FilterBlock>
        <label class="block text-sm font-medium text-gray-800 mb-2">Сортировать по:</label>
        <Dropdown v-model="sortBy" :options="sortOptions" optionLabel="name" optionValue="value" />
      </FilterBlock>
    </template>
  </PageActions>
  <GridBlock>
    <NodeList :sort-by="sortBy" @set-count="nodesCount = $event" />
  </GridBlock>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";

import Dropdown from "primevue/dropdown";
import FilterBlock from "@/components/common/filter/FilterBlock.vue";
import GridBlock from "@/components/common/grid/GridBlock.vue";
import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";

import NodeList from "@/components/node/NodeList.vue";
// import Breadcrumb from 'primevue/breadcrumb';

const route = useRoute();
const sortOptions = [
  { name: "Время создания", value: "creationTimestamp" },
  { name: "Имя", value: "name" },
];
const sortBy = ref<string>("name");

// const breadcrumbItems = ref(route.meta.breadcrumbs(route.params.ng_name));
// watch(
//   () => route.name,
//   (newVal) => {
//     if (['NodeListAll', 'NodeList'].indexOf(newVal as string) < 0) return;
//     breadcrumbItems.value = route.meta.breadcrumbs(route.params.ng_name);
//   }
// );

const nodesCount = ref<number | null>(null);
const tabs = route.params.ng_name
  ? [
      {
        title: "Просмотр",
        routeName: "NodeGroupShow",
        routeParams: { name: route.params.ng_name },
      },
      {
        title: "Редактирование",
        routeName: "NodeGroupEdit",
        routeParams: { name: route.params.ng_name },
      },
      {
        title: "Список узлов",
        badge: nodesCount,
        routeName: "NodeList",
        routeParams: { ng_name: route.params.ng_name },
      },
    ]
  : [];

console.log(route.params);
</script>

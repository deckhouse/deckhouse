<template>
  <PageTitle>{{ route.params.ng_name || "Узлы всех групп" }}</PageTitle>
  <PageActions>
    <template #tabs v-if="tabs">
      <TabsBlock :items="tabs" />
    </template>
    <template #filter>
      <FilterBlock>
        <label class="block text-sm font-medium text-gray-800 mb-2">Сортировать по:</label>
        <Dropdown
          :options="sortOptions"
          optionLabel="name"
          optionValue="value"
          v-model="sortBy"
          @change="$router.push({ query: { sortBy: $event.value } })"
        />
      </FilterBlock>
    </template>
  </PageActions>
  <GridBlock>
    <NodeList />
  </GridBlock>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { useRoute } from "vue-router";

import Node from "@/models/Node";

import useLoadAll from "@/composables/useLoadAll";

import Dropdown from "primevue/dropdown";
import FilterBlock from "@/components/common/filter/FilterBlock.vue";
import GridBlock from "@/components/common/grid/GridBlock.vue";
import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";

import NodeList from "@/components/node/NodeList.vue";

const route = useRoute();
const sortOptions = [
  { name: "Время создания (сначала новые)", value: "creationTimestamp" },
  { name: "Имя", value: "name" },
];

const sortBy = ref(route.query.sortBy?.toString() || "name");

const nodesCount = ref<number | null>(null);
const tabs = computed(() =>
  route.params.ng_name
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
          badge: nodesCount.value,
          routeName: "NodeList",
          routeParams: { ng_name: route.params.ng_name },
        },
      ]
    : []
);

useLoadAll(() => (nodesCount.value = Node.filterByNodeGroup(route.params.ng_name?.toString()).length));
</script>

<template>
  <PageTitle>The-biggest-node-group-name-imaginable-is-63-symbols-max-woohoo</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
    <template #filter>
      <FilterBlock></FilterBlock>
    </template>
  </PageActions>
  <GridBlock>
    <NodeList />
  </GridBlock>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { useRoute } from "vue-router";

import FilterBlock from "@/components/common/filter/FilterBlock.vue";
import GridBlock from "@/components/common/grid/GridBlock.vue";
import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";

import NodeList from "@/components/node/NodeList.vue";

const route = useRoute();
const tabs = [
  {
    id: "1",
    title: "Просмотр",
    routeName: "home",
  },
  {
    id: "2",
    title: "Редактирование",
    routeName: "home",
  },
  {
    id: "3",
    title: "Список узлов",
    active: true,
    badge: "3",
    routeName: "home",
  },
];

console.log(route.params);
</script>

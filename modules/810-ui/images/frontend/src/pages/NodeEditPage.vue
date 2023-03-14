<template>
  <PageTitle>The-biggest-node-group-name-imaginable-is-63-symbols-max-woohoo</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <NodeForm :name="route.params.name.toString()" />
</template>

<script setup lang="ts">
import { useRoute } from "vue-router";

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";

import NodeForm from "@/components/node/NodeForm.vue";

const route = useRoute();
const tabs = [
  {
    id: "1",
    title: "Просмотр",
    routeName: "NodeShow",
  },
  {
    id: "2",
    title: "Редактирование",
    active: true,
    routeName: "NodeEdit",
  },
];
</script>

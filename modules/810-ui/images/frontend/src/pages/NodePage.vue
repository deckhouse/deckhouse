<template>
  <PageTitle v-if="!isLoading">{{ node?.metadata.name }}</PageTitle>
  <Skeleton v-if="isLoading" class="mb-6" height="2rem" />
  <PageActions>
    <template #tabs v-if="!isLoading">
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <NodeForm v-if="node" :readonly="!isEdit" :node="node" />
  <CardBlock v-if="isLoading" :content-loading="isLoading" />
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { useRoute } from "vue-router";

import Skeleton from "primevue/skeleton";

import Node from "@/models/Node";

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";

import NodeForm from "@/components/node/NodeForm.vue";

const route = useRoute();

const isEdit = computed(() => route.name == "NodeEdit");

const isLoading = ref(false);
const node = ref<Node>();

const tabs = [
  {
    id: "1",
    title: "Просмотр",
    routeName: "NodeShow",
  },
  {
    id: "2",
    title: "Редактирование",
    routeName: "NodeEdit",
  },
];

function reload(): void {
  isLoading.value = true;
  Node.get({ name: route.params.name }).then((res: Node | null): void => {
    if (res) node.value = res;

    isLoading.value = false;
  });
}
reload();
</script>

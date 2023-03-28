<template>
  <PageTitle v-if="!isLoading && nodeGroup && !nodeGroup.isNew">{{ nodeGroup.name }}</PageTitle>
  <PageTitle v-if="!isLoading && nodeGroup?.isNew">Добавление группы типа {{ nodeGroup.spec.nodeType }}</PageTitle>
  <Skeleton v-if="isLoading" class="mb-6" height="2rem" />
  <PageActions>
    <template #tabs v-if="tabs.length">
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>

  <NodeGroupForm v-if="!isLoading && nodeGroup" :readonly="!isEdit && !isNew" :item="nodeGroup" />
  <CardBlock v-if="isLoading" :content-loading="isLoading" />

  <ErrorPage v-if="loadError" :load-error="loadError" />
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { useRoute } from "vue-router";

import NodeGroup, { type NodeTypesType } from "@/models/NodeGroup";
import Node from "@/models/Node";
import type { LoadError, TabsItem } from "@/types";

import useLoadAll from "@/composables/useLoadAll";

import Skeleton from "primevue/skeleton";

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";

import NodeGroupForm from "@/components/node_group/NodeGroupForm.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";
import ErrorPage from "@/pages/ErrorPage.vue";

const route = useRoute();

const loadError = ref<LoadError | undefined>();

const nodeGroup = ref<NodeGroup>();
const isEdit = computed(() => route.name == "NodeGroupEdit");
const isNew = computed(() => route.name == "NodeGroupNew");

const { isLoading } = useLoadAll(() => {
  if (isNew.value) {
    const nodeType = route.query.type ? (route.query.type.toString() as NodeTypesType) : "CloudEphemeral";
    nodeGroup.value = new NodeGroup({ isNew: true, spec: { nodeType }, metadata: { name: "" } }); // TODO: validate type param
  } else {
    nodeGroup.value = NodeGroup.find_with((model: NodeGroup) => model.name == route.params.name);

    if (!nodeGroup.value) {
      loadError.value = { code: 404, text: "Не найдено." };
    }
  }
});

const tabs = computed(() => {
  let res: TabsItem[] = [];

  if (isNew.value || !nodeGroup.value || isLoading.value) return res;

  res.push({
    title: "Просмотр",
    routeName: "NodeGroupShow",
  });

  res.push({
    title: "Редактирование",
    routeName: "NodeGroupEdit",
    disabled: nodeGroup.value?.isDeleting, //TODO: not working
  });

  res.push({
    title: "Список узлов",
    routeName: "NodeList",
    badge: Node.filterByNodeGroup(route.params.name.toString()).length,
    routeParams: { ng_name: route.params.name },
  });

  return res;
});
</script>

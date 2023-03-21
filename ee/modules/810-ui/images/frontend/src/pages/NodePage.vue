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
import { ref, computed, watch, watchEffect } from "vue";
import { useRoute } from "vue-router";

import Skeleton from "primevue/skeleton";

import Node from "@/models/Node";

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";

import NodeForm from "@/components/node/NodeForm.vue";
// import Breadcrumb from 'primevue/breadcrumb';
// const breadcrumbItems = ref([]);

const route = useRoute();
const isEdit = computed(() => route.name == "NodeEdit");
const isLoading = ref(false);
const node = ref<Node>();

// watch(
//   () => route.name,
//   () => { if (node.value) breadcrumbItems.value = route.meta.breadcrumbs(route.params.ng_name, node.value); },
//   { flush: 'post' }
// );

const tabs = [
  {
    title: "Просмотр",
    routeName: "NodeShow",
  },
  {
    title: "Редактирование",
    routeName: "NodeEdit",
  },
];

function reload(): void {
  isLoading.value = true;
  Node.get({ name: route.params.name }).then((res: Node | null): void => {
    if (res) {
      node.value = res;
      // breadcrumbItems.value = route.meta.breadcrumbs(route.params.ng_name, node.value);
    }

    isLoading.value = false;
  });
}
reload();
</script>

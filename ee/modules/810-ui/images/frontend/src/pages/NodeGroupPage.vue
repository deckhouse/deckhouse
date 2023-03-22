<template>
  <PageTitle v-if="!isLoading && nodeGroup && !nodeGroup.isNew">{{ nodeGroup.name }}</PageTitle>
  <PageTitle v-if="!isLoading && nodeGroup?.isNew">Добавление группы типа {{ nodeGroup.spec.nodeType }}</PageTitle>
  <Skeleton v-if="isLoading" class="mb-6" height="2rem" />
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>

  <NodeGroupForm v-if="!isLoading && nodeGroup" :readonly="!isEdit && !isNew" :item="nodeGroup" />
  <CardBlock v-if="isLoading" :content-loading="isLoading" />
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from "vue";
import { useRoute } from "vue-router";

import NodeGroup, { type NodeTypesType } from "@/models/NodeGroup";
import Discovery from "@/models/Discovery";

import Skeleton from "primevue/skeleton";

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";

import NodeGroupForm from "@/components/node_group/NodeGroupForm.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";

// TODO: one "type" of tabs = one object with one list
import Node from "@/models/Node";
import useListDynamic from "@lib/nxn-common/composables/useListDynamic";
import type { TabsItem } from "@/types";

const route = useRoute();

const nodesCount = ref<number | null>(null);
function resetCount() {
  nodesCount.value = list.items.length;
}
const list = useListDynamic<Node>(
  Node,
  {
    onLoadSuccess: resetCount,
    afterAdd: resetCount,
    afterRemove: resetCount,
    onLoadError: (error: any) => {
      console.error("Failed to load counts: " + JSON.stringify(error));
    },
  },
  {},
  { nodeGroupName: route.params.ng_name }
);
list.activate();
onBeforeUnmount(() => list.destroyList());

const isLoading = ref(true);
const nodeGroup = ref<NodeGroup>();
const isEdit = computed(() => route.name == "NodeGroupEdit");
const isNew = computed(() => route.name == "NodeGroupNew");

const tabs = computed(() => {
  let res: TabsItem[] = [];

  if (isNew.value || !nodeGroup.value) return res;

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
    badge: nodesCount,
    routeParams: { ng_name: route.params.name },
  });

  return res;
});

Discovery.get()
  .instanceClassKlass.query()
  .then(() => {
    if (isNew.value) {
      console.log("NEW!!");

      const nodeType = route.query.type ? (route.query.type.toString() as NodeTypesType) : "CloudEphemeral";
      nodeGroup.value = new NodeGroup({ isNew: true, spec: { nodeType }, metadata: { name: "" } }); // TODO: validate type param
      isLoading.value = false;
    } else {
      NodeGroup.get({ name: route.params.name }).then((res: NodeGroup | null): void => {
        console.log("RESRES!", res);

        if (res) {
          nodeGroup.value = res;
          // breadcrumbItems.value = route.meta.breadcrumbs(nodeGroup.value);
        }
        isLoading.value = false;
      });
    }
  });
</script>

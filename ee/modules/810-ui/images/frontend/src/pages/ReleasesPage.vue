<template>
  <PageTitle>Обновления</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <GridBlock>
    <DeckhouseModuleSettings />

    <ReleaseItemsList @set-count="ReleaseItemsCount = $event" />
  </GridBlock>
</template>

<script setup lang="ts">
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import GridBlock from "@/components/common/grid/GridBlock.vue";
import DeckhouseModuleSettings from "@/components/releases/DeckhouseModuleSettings.vue";
import ReleaseItemsList from "@/components/releases/ReleaseItemsList.vue";
import { ref } from "vue";
import type { TabsItem } from "@/types";
// import Breadcrumb from 'primevue/breadcrumb';
// import { useRoute } from "vue-router";
// const breadcrumbItems = useRoute().meta.breadcrumbs();

const ReleaseItemsCount = ref<number | null>(null);
const tabs = [
  {
    title: "Версии",
    badge: ReleaseItemsCount,
    routeName: "Home",
  },
  {
    title: "Настройки обновлений",
    routeName: "DeckhouseSettings",
  },
] as Array<TabsItem>;
</script>

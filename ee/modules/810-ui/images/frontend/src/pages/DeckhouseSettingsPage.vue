<template>
  <PageTitle>Обновления</PageTitle>
  <PageActions>
    <template #tabs>
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <DeckhouseModuleSettingsForm :deckhouse-module-settings="deckhouseModuleSettings" v-if="!isLoading && deckhouseModuleSettings" />
  <CardBlock v-if="isLoading" :content-loading="isLoading" />
</template>

<script setup lang="ts">
import { ref, onBeforeUnmount } from "vue";

import type { ITabsItem } from "@/types";
import DeckhouseModuleSettings, { type IDeckhouseModuleRelease } from "@/models/DeckhouseModuleSettings";

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";

import DeckhouseModuleSettingsForm from "@/components/releases/DeckhouseModuleSettingsForm.vue";

import { useRoute } from "vue-router";
// import Breadcrumb from 'primevue/breadcrumb';
// const breadcrumbItems = useRoute().meta.breadcrumbs();

// TODO: one "type" of tabs = one object with one list
import DeckhouseRelease from "@/models/DeckhouseRelease";
import useListDynamic from "@lib/nxn-common/composables/useListDynamic";
const ReleaseItemsCount = ref<number | null>(null);
function resetCount() { ReleaseItemsCount.value = list.items.length; }
const list = useListDynamic<DeckhouseRelease>(
  DeckhouseRelease,
  {
    onLoadSuccess: resetCount,
    afterAdd: resetCount,
    afterRemove: resetCount,
    onLoadError: (error: any) => { console.error("Failed to load counts: " + JSON.stringify(error)); },
  },
  {}
);
list.activate();
onBeforeUnmount(() => list.destroyList() );
const tabs = [
  {
    title: "Версии",
    badge: ReleaseItemsCount,
    routeName: "Home",
  },
  {
    active: true,
    title: "Настройки обновлений",
    routeName: "DeckhouseSettings",
  },
] as Array<ITabsItem>;

const isLoading = ref(true);
const deckhouseModuleSettings = ref<DeckhouseModuleSettings>()
DeckhouseModuleSettings.get().then((res: DeckhouseModuleSettings) => {
  res.spec.settings.release ||= {} as IDeckhouseModuleRelease;
  res.spec.settings.release.notification ||= {};

  // deckhouseSettings.value = new DeckhouseSettings(res.spec.settings);
  deckhouseModuleSettings.value = res;
  // setValues(res.spec.settings);
  isLoading.value = false;

  // @ts-ignore
  // DeckhouseModuleSettings.subscribe(); // TODO: Alerts if smth change
});
</script>

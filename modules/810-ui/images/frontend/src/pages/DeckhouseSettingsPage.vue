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
import { ref } from "vue";

import type { ITabsItem } from "@/types";
import DeckhouseModuleSettings, { type IDeckhouseModuleRelease } from "@/models/DeckhouseModuleSettings";

import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";

import DeckhouseModuleSettingsForm from "@/components/releases/DeckhouseModuleSettingsForm.vue";

const tabs = [
  {
    id: "1",
    title: "Версии",
    badge: ref<number>(0),
    routeName: "home",
  },
  {
    id: "2",
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

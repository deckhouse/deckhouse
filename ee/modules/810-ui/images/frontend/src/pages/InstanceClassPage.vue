<template>
  <PageTitle v-if="!isLoading && item && !isNew">{{ item.name }}</PageTitle>
  <PageTitle v-if="isNew">Добавление класса машин</PageTitle>
  <Skeleton v-if="isLoading" class="mb-6" height="2rem" />
  <PageActions>
    <template #tabs v-if="!isNew">
      <TabsBlock :items="tabs" />
    </template>
  </PageActions>
  <component :is="itemForm" v-if="item" :readonly="!isEdit && !isNew" :item="item" />
  <CardBlock v-if="isLoading" :content-loading="isLoading" />

  <ErrorPage v-if="loadError" :load-error="loadError" />
</template>

<script setup lang="ts">
import { computed, ref, shallowRef } from "vue";
import { useRoute } from "vue-router";

import type { InstanceClassesTypes } from "@/models/instanceclasses";
import Discovery from "@/models/Discovery";
import type { LoadError, TabsItem } from "@/types";

import useLoadAll from "@/composables/useLoadAll";

import Skeleton from "primevue/skeleton";
import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";

import forms from "@/components/instanceclass/forms"; // different forms for different types
import ErrorPage from "./ErrorPage.vue";
// import Breadcrumb from 'primevue/breadcrumb';
// const breadcrumbItems = ref([]);

const route = useRoute();
const loadError = ref<LoadError | undefined>();

const discovery = Discovery.get();
const item = ref<InstanceClassesTypes>();
const itemForm = shallowRef();

const isNew = computed(() => route.name == "InstanceClassNew");
const isEdit = computed(() => route.name == "InstanceClassEdit");

const tabs = computed(() => {
  let res: TabsItem[] = [];

  if (isNew.value || !item.value || isLoading.value) return res;

  res = [
    {
      title: "Просмотр",
      routeName: "InstanceClassShow",
    },
    {
      title: "Редактирование",
      routeName: "InstanceClassEdit",
    },
  ];
  return res;
});

itemForm.value = forms[discovery.cloudProvider.name as keyof typeof forms];

const { isLoading } = useLoadAll(() => {
  if (isNew.value) {
    item.value = new discovery.instanceClassKlass({ isNew: true });
  } else {
    item.value = discovery.instanceClassKlass.find_with((val) => val.name == route.params.name.toString());

    if (!item.value) {
      loadError.value = { code: 404, text: "Не найдено." };
    }
  }
});
</script>

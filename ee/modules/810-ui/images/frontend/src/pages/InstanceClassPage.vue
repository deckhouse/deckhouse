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
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useRoute } from "vue-router";

import type { InstanceClassesTypes } from "@/models/instanceclasses";
import Discovery from "@/models/Discovery";

import Skeleton from "primevue/skeleton";
import PageTitle from "@/components/common/page/PageTitle.vue";
import PageActions from "@/components/common/page/PageActions.vue";
import TabsBlock from "@/components/common/tabs/TabsBlock.vue";
import CardBlock from "@/components/common/card/CardBlock.vue";

import forms from "@/components/instanceclass/forms"; // different forms for different types
// import Breadcrumb from 'primevue/breadcrumb';
// const breadcrumbItems = ref([]);

const route = useRoute();

const discovery = Discovery.get();
const item = ref<InstanceClassesTypes>();
const itemForm = shallowRef();

const isNew = computed(() => route.name == "InstanceClassNew");
const isEdit = computed(() => route.name == "InstanceClassEdit");

const isLoading = ref(!isNew.value);

// watch(
//   () => route.name,
//   () => { if (item.value) breadcrumbItems.value = route.meta.breadcrumbs(item.value); },
//   { flush: 'post' }
// );

const tabs = [
  {
    id: "1",
    title: "Просмотр",
    routeName: "InstanceClassShow",
  },
  {
    id: "2",
    title: "Редактирование",
    routeName: "InstanceClassEdit",
  },
];

itemForm.value = forms[discovery.cloudProvider.name as keyof typeof forms];

if (isNew.value) {
  item.value = new discovery.instanceClassKlass({ isNew: true });
} else {
  discovery.instanceClassKlass.get({ name: route.params.name }).then((mc: InstanceClassesTypes) => {
    // mc.instanceTypeInfo = discovery.value!.instanceTypeInfo;
    item.value = mc;
    // breadcrumbItems.value = route.meta.breadcrumbs(item.value);

    isLoading.value = false;
  });
}
</script>

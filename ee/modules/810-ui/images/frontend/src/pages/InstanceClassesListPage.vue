<template>
  <PageTitle>Классы машин</PageTitle>
  <PageActions>
    <template #actions>
      <ButtonBlock icon="IconInstall" title="Добавить" type="primary" @click="createItem" />
    </template>
    <template #filter>
      <FilterBlock>
        <label class="block text-sm font-medium text-gray-800 mb-2">Сортировать по:</label>
        <Dropdown v-model="sortBy" :options="sortOptions" optionLabel="name" optionValue="value" />
      </FilterBlock>
    </template>
  </PageActions>
  <GridBlock>
    <InstanceClassesList :sort-by="sortBy" />
  </GridBlock>
</template>

<script setup lang="ts">
import { ref } from "vue";

import Discovery from "@/models/Discovery";

import Dropdown from "primevue/dropdown";

import GridBlock from "../components/common/grid/GridBlock.vue";
import PageTitle from "../components/common/page/PageTitle.vue";
import PageActions from "../components/common/page/PageActions.vue";
import ButtonBlock from "../components/common/button/ButtonBlock.vue";
import InstanceClassesList from "@/components/instanceclass/InstanceClassesList.vue";
import FilterBlock from "@/components/common/filter/FilterBlock.vue";

import FlashMessagesService from "@/services/FlashMessagesService.js";
import FormatError from "@/services/FormatError.js";
import router from "@/router";

// import Breadcrumb from 'primevue/breadcrumb';
// const breadcrumbItems = useRoute().meta.breadcrumbs();

const sortOptions = [
  { name: "Время создания", value: "creationTimestamp" },
  { name: "Имя", value: "name" },
];
const sortBy = ref<string>("name");

const discovery = Discovery.get();

function createItem() {
  router.push({ name: "InstanceClassNew" });
}
</script>

<template>
  <PageTitle>Группы узлов</PageTitle>
  <PageActions>
    <template #filter>
      <FilterBlock>
        <label class="block text-sm font-medium text-gray-800 mb-2">Сортировать по:</label>
        <Dropdown v-model="sortBy" :options="sortOptions" optionLabel="name" optionValue="value" />
      </FilterBlock>
    </template>
    <template #actions>
      <ButtonBlock title="Cloud Ephemeral" type="primary" icon="IconPlus" @click="openCreationForm('CloudEphemeral')" />
      <ButtonBlock title="Cloud Static" type="primary" icon="IconPlus" @click="openCreationForm('CloudStatic')" />
      <ButtonBlock title="Static" type="primary" icon="IconPlus" @click="openCreationForm('Static')" />
    </template>
  </PageActions>
  <GridBlock>
    <NodeGroupList :sort-by="sortBy" />
  </GridBlock>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";

import Dropdown from "primevue/dropdown";
import FilterBlock from "../components/common/filter/FilterBlock.vue";
import GridBlock from "../components/common/grid/GridBlock.vue";
import PageTitle from "../components/common/page/PageTitle.vue";
import PageActions from "../components/common/page/PageActions.vue";
import ButtonBlock from "../components/common/button/ButtonBlock.vue";

import NodeGroupList from "../components/node_group/NodeGroupList.vue";

import FlashMessagesService from "@/services/FlashMessagesService.js";
import FormatError from "@/services/FormatError.js";

import { useRouter } from "vue-router";
// import Breadcrumb from 'primevue/breadcrumb';
// const breadcrumbItems = useRoute().meta.breadcrumbs();

const router = useRouter();

const sortOptions = [
  { name: "Время создания", value: "creationTimestamp" },
  { name: "Имя", value: "name" },
];
const sortBy = ref<string>("name");

function openCreationForm(type: string) {
  console.log(type);

  router.push({ name: "NodeGroupNew", query: { type: type } });
}
</script>

<template>
  <PageTitle>Группы узлов</PageTitle>
  <PageActions>
    <template #filter>
      <FilterBlock>
        <label class="block text-sm font-medium text-gray-800 mb-2">Сортировать по:</label>
        <Dropdown
          :options="sortOptions"
          optionLabel="name"
          optionValue="value"
          v-model="sortBy"
          @change="$router.push({ query: { sortBy: $event.value } })"
        />
      </FilterBlock>
    </template>
    <template #actions>
      <ButtonBlock title="Cloud Ephemeral" type="primary" icon="IconPlus" @click="openCreationForm('CloudEphemeral')" />
      <ButtonBlock title="Cloud Static" type="primary" icon="IconPlus" @click="openCreationForm('CloudStatic')" />
      <ButtonBlock title="Static" type="primary" icon="IconPlus" @click="openCreationForm('Static')" />
    </template>
  </PageActions>
  <GridBlock>
    <NodeGroupList />
  </GridBlock>
</template>

<script setup lang="ts">
import { ref } from "vue";

import Dropdown from "primevue/dropdown";
import FilterBlock from "../components/common/filter/FilterBlock.vue";
import GridBlock from "../components/common/grid/GridBlock.vue";
import PageTitle from "../components/common/page/PageTitle.vue";
import PageActions from "../components/common/page/PageActions.vue";
import ButtonBlock from "../components/common/button/ButtonBlock.vue";

import NodeGroupList from "../components/node_group/NodeGroupList.vue";

import { useRouter } from "vue-router";

const router = useRouter();

const sortOptions = [
  { name: "Время создания (сначала новые)", value: "creationTimestamp" },
  { name: "Имя", value: "name" },
];
const sortBy = ref(router.currentRoute.value.query.sortBy?.toString() || "name");

function openCreationForm(type: string) {
  router.push({ name: "NodeGroupNew", query: { type: type } });
}
</script>

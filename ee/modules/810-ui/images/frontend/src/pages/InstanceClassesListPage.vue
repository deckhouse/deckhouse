<template>
  <PageTitle>Классы машин</PageTitle>
  <PageActions>
    <template #actions>
      <ButtonBlock icon="IconInstall" title="Добавить" type="primary" @click="createItem" />
    </template>
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
  </PageActions>
  <GridBlock>
    <InstanceClassesList />
  </GridBlock>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";

import Dropdown from "primevue/dropdown";

import GridBlock from "../components/common/grid/GridBlock.vue";
import PageTitle from "../components/common/page/PageTitle.vue";
import PageActions from "../components/common/page/PageActions.vue";
import ButtonBlock from "../components/common/button/ButtonBlock.vue";
import InstanceClassesList from "@/components/instanceclass/InstanceClassesList.vue";
import FilterBlock from "@/components/common/filter/FilterBlock.vue";

const router = useRouter();

const sortOptions = [
  { name: "Время создания (сначала новые)", value: "creationTimestamp" },
  { name: "Имя", value: "name" },
];
const sortBy = ref(router.currentRoute.value.query.sortBy?.toString() || "name");

function createItem() {
  router.push({ name: "InstanceClassNew" });
}
</script>

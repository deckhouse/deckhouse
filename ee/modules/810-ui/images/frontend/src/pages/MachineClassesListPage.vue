<template>
  <PageTitle>Классы машин</PageTitle>
  <PageActions>
    <template #actions>
      <ButtonBlock icon="IconInstall" title="Добавить" type="primary" />
    </template>
  </PageActions>
  <GridBlock>
    <MachineClassesListItem></MachineClassesListItem>
  </GridBlock>
</template>

<script setup lang="ts">
import GridBlock from "../components/common/grid/GridBlock.vue";
import PageTitle from "../components/common/page/PageTitle.vue";
import PageActions from "../components/common/page/PageActions.vue";
import ButtonBlock from "../components/common/button/ButtonBlock.vue";
import MachineClassesListItem from "@/components/machine_class/MachineClassesListItem.vue";
</script>

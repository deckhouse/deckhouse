<template>
  <PageTitle>Группы узлов</PageTitle>
  <PageActions>
    <template #filter>
      <FilterBlock></FilterBlock>
    </template>
    <template #actions>
      <ButtonBlock title="Создать группу" type="primary" />
    </template>
  </PageActions>
  <GridBlock>
    <NodeGroupListItem></NodeGroupListItem>
    <NodeGroupListItem></NodeGroupListItem>
  </GridBlock>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';

import FilterBlock from "../components/common/filter/FilterBlock.vue";
import GridBlock from "../components/common/grid/GridBlock.vue";
import PageTitle from "../components/common/page/PageTitle.vue";
import PageActions from "../components/common/page/PageActions.vue";
import ButtonBlock from "../components/common/button/ButtonBlock.vue";

import NodeGroupListItem from "../components/node_group/NodeGroupListItem.vue";
</script>

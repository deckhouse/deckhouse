<template>
  <ButtonBlock
    v-if="showEdit"
    title="Редактировать"
    type="primary-subtle"
    :disabled="item.isDeleting"
    :route="{ name: 'NodeGroupEdit', params: { name: item.name } }"
  ></ButtonBlock>
  <ButtonBlock
    title="Удалить"
    type="danger-subtle"
    @click="handleDelete"
    :loading="deleteLoading || !!item.isDeleting"
    :disabled="item.spec.nodeType == 'CloudPermanent'"
  ></ButtonBlock>
</template>

<script setup lang="ts">
import { ref, type PropType } from "vue";

import type NodeGroup from "@/models/NodeGroup";

import ButtonBlock from "@/components/common/button/ButtonBlock.vue";

const props = defineProps({
  item: {
    type: Object as PropType<NodeGroup>,
    required: true,
  },
  showEdit: {
    type: Boolean,
    default: true,
  },
});

const deleteLoading = ref(false);

function handleDelete() {
  deleteLoading.value = true;
  props.item.delete().then((res) => {
    console.log("DELETE RES", res);

    deleteLoading.value = false;
  });
}
</script>

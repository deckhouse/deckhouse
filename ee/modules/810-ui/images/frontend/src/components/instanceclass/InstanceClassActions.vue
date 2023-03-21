<template>
  <ButtonBlock title="Удалить" type="danger-subtle" @click="deleteItem" :loading="deleteLoading"></ButtonBlock>
</template>

<script setup lang="ts">
import { ref, type PropType } from "vue";
import { useRouter } from "vue-router";

import type InstanceClassBase from "@/models/instanceclasses/InstanceClassBase";

import ButtonBlock from "@/components/common/button/ButtonBlock.vue";

const router = useRouter();

const props = defineProps({
  item: {
    type: Object as PropType<InstanceClassBase>,
    required: true,
  },
});

const deleteLoading = ref(false);

function deleteItem() {
  deleteLoading.value = true;
  props.item.delete().then(() => {
    console.log("DELETED!");
    deleteLoading.value = false;
    router.push({ name: "InstanceClassesList" });
  });
}
</script>

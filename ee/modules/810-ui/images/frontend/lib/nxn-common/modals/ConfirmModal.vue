<template>
  <vue-final-modal
    v-slot="{ close }"
    classes="modal-container nxn-modal"
    content-class="modal-content nxn-modal__content"
    :content-style="'width: 500px'"
    :esc-to-close="true"
    :z-index-base="9990"
    :focus-retain="false"
    :focus-trap="false">
    <span class="modal__title">{{ title }}</span>

    <div class="modal__action">
      <button class="btn btn-primary"
              @click="$emit('confirm'); close()"
              @keydown.enter.prevent="$emit('confirm'); close()"
              ref="confirmButton">
        Ok
      </button>
      <button class="ml-2 btn btn-secondary" @click="close">
        <span v-once>{{ translate('common.cancel') }}</span>
      </button>
    </div>
  </vue-final-modal>
</template>

<script>
export default {
  emits: ['confirm'],

  props: {
    title: String
  },

  mounted() {
    this.$nextTick(()=> this.$refs.confirmButton.focus());
  }
}
</script>

<template>
  <div id="f-flash-container">
    <div v-for="message in messages">
      <div class="flash-message">
        <div :class="['f-flash', 'f-flash-' + message.type]">
          <span class="type">[{{ message.type }}!]</span>
          {{ message.text }}
          <button v-if="!message.isReloader" type="button" class="close" @click="closeMessage(message.key)">
            <span aria-hidden="true">×</span>
            <span class="sr-only">Close</span>
          </button>
          <div v-if="message.isReloader" style="margin-top: 10px;">
            <button @click="closeMessage(message.key)"
                    type="button"
                    class="btn btn-xs btn-primary"
                    v-tooltip.bottom="translateFilter('flash_messages.require_reload')">
              {{ translate('flash_messages.reload_btn') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import translateFilter from 'nxn-common/filters/translate.js';
import { watch, reactive } from 'vue';

export default {
  inject: ['$eventBus'],
  
  data() {
    const messages = reactive([]);
    return {
      messages
    };
  },

  created() {
    this.$eventBus.on('NxnFlash::add', this.addMessage); // never offed
    this.$eventBus.on('NxnFlash::close', this.closeKey); // never offed
  },

  methods: {
    translateFilter: translateFilter,

    addMessage(message) {
      if (message.key && this.idxOfKey(message.key) > -1) this.closeKey(message.key);

      this.messages.push(message);
      var vm = this;
      if (message.timeout) setTimeout(function(){ vm.closeMessage(message.key); }, message.timeout);
    },

    idxOfKey(key) {
      return this.messages.findIndex(function(message) { return message.key == key; });
    },

    closeMessage(key) {
      var idx = this.idxOfKey(key);
      var message = this.messages[idx];
      this.closeIdx(idx);
      if (message && message.isReloader) window.location.href = location.href;
    },

    closeKey(key) {
      if (typeof key == 'undefined') return;
      this.closeIdx(this.idxOfKey(key));
    },

    closeIdx(idx) {
      if (idx > -1) this.messages.splice(idx, 1);
    }
  }
}
</script>

<style>
#f-flash-container {
  width: 100%;
  z-index: 3000;
  position: fixed;
  bottom: 0px;
  left: 0px;
  background: #222;
  opacity: 0.95;
  font-size: 14px;
}

@media (min-width: 1000px) {
  #f-flash-container {
    max-width: 1000px;
    left: 50%;
    margin-left: -500px;
  }
}

#f-flash-container .f-flash {
  padding: 10px 10px 10px 10px;
  color: #f5f5f5;
}

#f-flash-container .f-flash .type {
  text-transform: uppercase;
}

#f-flash-container .f-flash-error .type {
  background: #8A1717;
}

#f-flash-container .f-flash-warning .type {
  background: #8A5717;
}

#f-flash-container .f-flash-success .type {
  background: #10874A;
}
/*
#f-flash-container .flash-message .f-flash {
  display: none;
}
*/
#f-flash-container .close {
  color: #fff;
}
</style>

<template>
  <span v-tooltip.top="dateFilter(gmtTimestamp * 1000, 'Z z')">
    {{ $filters.date(gmtTimestamp * 1000, format) }}
  </span>
</template>

<script>
import dateFilter from 'nxn-common/filters/date.js';

export default {
  props: {
    gmtTimestamp: Number,
    withYear: { type: Boolean, default: false }
  },

  data() {
    return {
      format: `DD/MM${(this.withYear || !this.isSameYear(this.gmtTimestamp)) ? '/YY' : ''} HH:mm:ss`
    }
  },

  methods: {
    dateFilter: dateFilter,
    isSameYear: (timestamp) => {
      return new Date(timestamp * 1000).getFullYear() == new Date().getFullYear()
    }
  }
};
</script>

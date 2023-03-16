var ListRenderLimit = {
  data() {
    return {
      listRenderLimit: 30,
      listRenderLimitDefaultIncrement: 30
    };
  },

  computed: {
    listRenderLimitIncrement() {
      if (this.items.length <= this.listRenderLimit) return 0;
      return Math.min(this.listRenderLimitDefaultIncrement, this.items.length - this.listRenderLimit);
    }
  },

  methods: {
    renderMoreItems() {
      this.listRenderLimit += this.listRenderLimitIncrement;
    },
    renderAllItems() {
      this.listRenderLimit = this.items.length;
    }
  }
};

export default ListRenderLimit;

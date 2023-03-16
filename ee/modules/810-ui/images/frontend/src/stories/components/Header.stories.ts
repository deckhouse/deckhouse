import type { Meta, Story } from "@storybook/vue3";

import HeaderComponent from "@/components/header/TheHeader.vue";

export default {
  title: "Deckhouse UI/Components/Header",
  component: HeaderComponent,
  parameters: { layout: "fullscreen" },
} as Meta;

export const Default: Story = (args) => ({
  components: { HeaderComponent },
  setup() {
    return { args };
  },
  template: '<HeaderComponent v-bind="args" />',
});

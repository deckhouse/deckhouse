import type { Meta, Story } from "@storybook/vue3";

import HeaderComponent from "@/components/header/TheHeader.vue";
import { routerDecorator } from "../common";

export default {
  title: "Deckhouse UI/Components/Header",
  component: HeaderComponent,
  parameters: { layout: "fullscreen" },
  decorators: [routerDecorator],
} as Meta;

export const Default: Story = (args) => ({
  components: { HeaderComponent },
  setup() {
    return { args };
  },
  template: '<HeaderComponent v-bind="args" />',
});

Default.parameters = {
  router: {
    currentRoute: { name: "Home" },
  },
};

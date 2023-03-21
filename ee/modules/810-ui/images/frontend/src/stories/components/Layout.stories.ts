import type { Meta, Story } from "@storybook/vue3";
import BaseLayoutComponent from "@/components/layout/BaseLayout.vue";
import { routerDecorator } from "../common";

export default {
  title: "Deckhouse UI/Components/Layout",
  component: BaseLayoutComponent,
  parameters: { layout: "fullscreen" },
  decorators: [routerDecorator],
} as Meta;

const Template: Story = (args) => ({
  components: { BaseLayoutComponent },
  setup() {
    return { args };
  },
  template: '<BaseLayoutComponent v-bind="args" />',
});

export const Base = Template.bind({});
Base.parameters = {
  router: {
    currentRoute: { name: "Home" },
  },
};

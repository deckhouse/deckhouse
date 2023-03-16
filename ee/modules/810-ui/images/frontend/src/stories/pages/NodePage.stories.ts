import type { Meta, Story } from "@storybook/vue3";

import { routerDecorator } from "../common";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import NodePage from "@/pages/NodePage.vue";

export default {
  title: "Deckhouse UI/Pages/Node",
  component: NodePage,
  parameters: { layout: "fullscreen" },
  decorators: [routerDecorator],
} as Meta;

const Template: Story = (args) => ({
  components: { NodePage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <NodePage v-bind="args"/>
    </BaseLayout>
  `,
});

export const Default = Template.bind({});

Default.parameters = {
  router: {
    currentRoute: { name: "NodeShow", params: { ng_name: "all", name: "kube-system-5bb6d73e-58dbf-f8t4t" } },
  },
};

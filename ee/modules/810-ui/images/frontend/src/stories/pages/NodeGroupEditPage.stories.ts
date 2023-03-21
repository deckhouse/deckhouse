import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import NodeGroupPage from "@/pages/NodeGroupPage.vue";
import { routerDecorator } from "../common";

export default {
  title: "Deckhouse UI/Pages/Node Group/Edit",
  component: NodeGroupPage,
  parameters: { layout: "fullscreen" },
  decorators: [routerDecorator],
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { NodeGroupPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <NodeGroupPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
Default.parameters = {
  router: {
    currentRoute: { name: "NodeGroupEdit", params: { name: "another-muffin-console" } },
  },
};

import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import NodeGroupListEditPage from "../../pages/NodeGroupListEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Node Group List Edit",
  component: NodeGroupListEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { NodeGroupListEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <NodeGroupListEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
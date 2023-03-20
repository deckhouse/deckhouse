import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import NodeGroupListPage from "../../pages/NodeGroupListPage.vue";

export default {
  title: "Deckhouse UI/Pages/Node Group/List",
  component: NodeGroupListPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { NodeGroupListPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <NodeGroupListPage/>
    </BaseLayout>
  `,
});

export const Default = Template.bind({});

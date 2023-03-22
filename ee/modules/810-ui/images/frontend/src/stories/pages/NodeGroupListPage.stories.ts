import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import NodeGroupListPage from "@/pages/NodeGroupListPage.vue";

import NodeGroup from "@/models/NodeGroup";

import { rest } from "msw";

export default {
  title: "Deckhouse UI/Pages/Node Group/List",
  component: NodeGroupListPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args) => ({
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

export const Empty = Template.bind({});

Empty.parameters = {
  msw: {
    handlers: {
      releases: [
        rest.get(NodeGroup.apiUrl("k8s/deckhouse.io/nodegroups"), (req, res, ctx) => {
          return res(ctx.delay(500), ctx.json([]));
        }),
      ],
    },
  },
};

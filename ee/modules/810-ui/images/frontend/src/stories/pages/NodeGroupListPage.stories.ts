import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import NodeGroupListPage from "@/pages/NodeGroupListPage.vue";

import NodeGroup from "@/models/NodeGroup";

import { rest } from "msw";
import { routerDecorator } from "../common";

export default {
  title: "Deckhouse UI/Pages/Node Group/List",
  component: NodeGroupListPage,
  parameters: {
    layout: "fullscreen",
    router: {
      currentRoute: { name: "NodeGroupList" }, // need route for correct work of useLoadAll
    },
  },
  decorators: [routerDecorator],
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
      nodeGroups: [
        rest.get(NodeGroup.apiUrl("k8s/deckhouse.io/nodegroups"), (req, res, ctx) => {
          return res(ctx.delay(500), ctx.json([]));
        }),
      ],
    },
  },
};

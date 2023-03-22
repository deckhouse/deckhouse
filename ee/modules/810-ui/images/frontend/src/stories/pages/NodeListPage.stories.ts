import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import NodeListPage from "@/pages/NodeListPage.vue";
import { routerDecorator } from "../common";
import Node from "@/models/Node";
import { rest } from "msw";

export default {
  title: "Deckhouse UI/Pages/Node/List",
  component: NodeListPage,
  parameters: { layout: "fullscreen", router: { currentRoute: { name: "NodeListAll" } } },
  decorators: [routerDecorator],
} as Meta;

const Template: Story = (args) => ({
  components: { NodeListPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <NodeListPage/>
    </BaseLayout>
  `,
});

export const Default = Template.bind({});

export const Empty = Template.bind({});

Empty.parameters = {
  msw: {
    handlers: {
      releases: [
        rest.get(Node.apiUrl("k8s/nodes"), (req, res, ctx) => {
          return res(ctx.delay(500), ctx.json([]));
        }),
      ],
    },
  },
};

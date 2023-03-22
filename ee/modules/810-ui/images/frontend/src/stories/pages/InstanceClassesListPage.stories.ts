import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import InstanceClassesListPage from "@/pages/InstanceClassesListPage.vue";

import InstanceClassBase from "@/models/instanceclasses/InstanceClassBase";

import { rest } from "msw";

export default {
  title: "Deckhouse UI/Pages/Instance Classes/List",
  component: InstanceClassesListPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { InstanceClassesListPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <InstanceClassesListPage/>
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
export const Empty = Template.bind({});

Empty.parameters = {
  msw: {
    handlers: {
      releases: [
        rest.get(InstanceClassBase.apiUrl("k8s/deckhouse.io/awsinstanceclasses"), (req, res, ctx) => {
          return res(ctx.delay(500), ctx.json([]));
        }),
      ],
    },
  },
};

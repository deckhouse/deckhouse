import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import InstanceClassesListPage from "@/pages/InstanceClassesListPage.vue";

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

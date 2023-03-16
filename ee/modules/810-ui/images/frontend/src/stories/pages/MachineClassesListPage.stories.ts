import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import MachineClassesListPage from "../../pages/MachineClassesListPage.vue";

export default {
  title: "Deckhouse UI/Pages/Machine Classes List",
  component: MachineClassesListPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { MachineClassesListPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <MachineClassesListPage/>
    </BaseLayout>
  `,
});

export const Default = Template.bind({});

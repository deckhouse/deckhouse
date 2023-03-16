import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import MachineClassAzureEditPage from "@/pages/MachineClassAzureEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Machine Class Azure Edit",
  component: MachineClassAzureEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { MachineClassAzureEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <MachineClassAzureEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
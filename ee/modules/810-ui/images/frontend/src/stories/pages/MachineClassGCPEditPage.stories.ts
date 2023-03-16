import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import MachineClassGCPEditPage from "@/pages/MachineClassGCPEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Machine Class GCP Edit",
  component: MachineClassGCPEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { MachineClassGCPEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <MachineClassGCPEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
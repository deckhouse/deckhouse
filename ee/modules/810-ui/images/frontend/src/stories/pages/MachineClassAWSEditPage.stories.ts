import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import MachineClassAWSEditPage from "../../pages/MachineClassAWSEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Machine Class AWS Edit",
  component: MachineClassAWSEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { MachineClassAWSEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <MachineClassAWSEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
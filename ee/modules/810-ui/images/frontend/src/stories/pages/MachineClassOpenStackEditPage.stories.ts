import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import MachineClassOpenStackEditPage from "../../pages/MachineClassOpenStackEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Instance Classes/Open Stack Edit",
  component: MachineClassOpenStackEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { MachineClassOpenStackEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <MachineClassOpenStackEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
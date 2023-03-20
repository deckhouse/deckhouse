import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import MachineClassVmWareEditPage from "../../pages/MachineClassVmWareEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Instance Classes/VmWare Edit",
  component: MachineClassVmWareEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { MachineClassVmWareEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <MachineClassVmWareEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});
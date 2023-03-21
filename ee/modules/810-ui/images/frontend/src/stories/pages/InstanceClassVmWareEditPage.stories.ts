import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import InstanceClassVmWareEditPage from "../../pages/instanceclass/InstanceClassVmWareEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Instance Classes/VmWare Edit",
  component: InstanceClassVmWareEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { InstanceClassVmWareEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <InstanceClassVmWareEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});

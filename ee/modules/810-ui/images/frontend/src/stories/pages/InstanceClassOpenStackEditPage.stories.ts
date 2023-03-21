import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import InstanceClassOpenStackEditPage from "../../pages/instanceclass/InstanceClassOpenStackEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Instance Classes/Open Stack Edit",
  component: InstanceClassOpenStackEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { InstanceClassOpenStackEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <InstanceClassOpenStackEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});

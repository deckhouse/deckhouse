import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "../../components/layout/BaseLayout.vue";
import InstanceClassAWSEditPage from "../../pages/instanceclass/InstanceClassAWSEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Instance Classes/AWS Edit",
  component: InstanceClassAWSEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { InstanceClassAWSEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <InstanceClassAWSEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});

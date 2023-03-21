import type { Meta, Story } from "@storybook/vue3";

import BaseLayout from "@/components/layout/BaseLayout.vue";
import InstanceClassYandexCloudEditPage from "@/pages/instanceclass/InstanceClassYandexCloudEditPage.vue";

export default {
  title: "Deckhouse UI/Pages/Instance Classes/Yandex Cloud Edit",
  component: InstanceClassYandexCloudEditPage,
  parameters: { layout: "fullscreen" },
} as Meta;

const Template: Story = (args, { loaded: { releases } }) => ({
  components: { InstanceClassYandexCloudEditPage, BaseLayout },

  setup() {
    return { args };
  },

  template: `
    <BaseLayout :compact="false">
      <InstanceClassYandexCloudEditPage />
    </BaseLayout>
  `,
});

export const Default = Template.bind({});

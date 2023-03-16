import type { Meta, Story } from "@storybook/vue3";

import TabsBlock from "@/components/common/tabs/TabsBlock.vue";

export default {
  title: "Deckhouse UI/Components/Tabs",
  component: TabsBlock,
  parameters: { layout: "fullscreen" },
} as Meta;


const Template: Story = (args) => ({
  components: { TabsBlock },
  setup() {
    return { args };
  },
  template: '<TabsBlock v-bind="args" />',
});

export const Default = Template.bind({});
Default.args = {
  items: [
    {
      id: "1",
      title: "Версии",
      active: true,
      badge: "3+",
      routeName: "home",
    },
    {
      id: "2",
      title: "Настройки обновлений",
      routeName: "DeckhouseSettings",
    },
  ],
};

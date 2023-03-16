import * as Icons from "@/components/common/icon";
import type { Meta, Story } from "@storybook/vue3";

const IconNames = Object.keys(Icons);
const IconOptions = {
  options: IconNames,
  defaultValue: IconNames[0],
  control: { type: "select" },
};

export default {
  title: "Deckhouse UI/Components/Icon",
  argTypes: {
    icon: IconOptions,
  },
} as Meta;

export const Icon: Story = (args) => ({
  components: { ...Icons },
  setup() {
    return { args };
  },
  template: `<${args.icon} />`,
});

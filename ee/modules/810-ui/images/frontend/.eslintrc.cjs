/* eslint-env node */
require("@rushstack/eslint-patch/modern-module-resolution");

module.exports = {
  root: true,
  extends: [
    "plugin:vue/vue3-essential",
    "eslint:recommended",
    "@vue/eslint-config-typescript",
    "@vue/eslint-config-prettier",
    "plugin:storybook/recommended",
  ],
  parserOptions: {
    ecmaVersion: "latest",
  },
  overrides: [
    {
      files: ["*.config.js"],
      env: {
        node: true,
      },
    },
    {
      files: ["*.ts", "*.vue"],
      rules: {
        "no-undef": "off",
        "prettier/prettier": [
          "error",
          {
            tabWidth: 2,
            useTabs: false,
            printWidth: 140,
          },
        ],
      },
    },
  ],
};

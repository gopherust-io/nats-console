import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: [
      "dist",
      "playwright-report",
      "test-results",
      "e2e",
      "playwright.config.ts",
      "vitest.config.ts",
    ],
  },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": [
        "warn",
        {
          allowConstantExport: true,
          allowExportNames: [
            "useAuth",
            "useCluster",
            "useAccount",
            "useTheme",
            "useToast",
            "applyTheme",
            "THEMES",
            "THEME_IDS",
            "DEFAULT_THEME",
            "clusterQueryKey",
            "QueryProvider",
            "pageSizeOptions",
            "DEFAULT_PAGE_SIZE",
            "pageQuery",
          ],
        },
      ],
    },
  },
  {
    files: ["src/test/**/*.{ts,tsx}", "src/**/*.{test,spec}.{ts,tsx}"],
    rules: {
      "react-refresh/only-export-components": "off",
    },
  },
);

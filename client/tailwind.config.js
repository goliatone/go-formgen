/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./dev/**/*.{html,js,ts,jsx,tsx}",
    "./src/**/*.{js,ts,jsx,tsx}",
    "./tests/**/*.{js,ts,jsx,tsx}",
    "./data/**/*.{json,html}",
    "../examples/http/**/*.{go,html,tmpl}",
    "../pkg/renderers/**/*.{tmpl,html}",
    "../pkg/renderers/vanilla/testdata/**/*.{html,json}",
    "../pkg/uischema/**/*.{json,yml,yaml}",
    "./node_modules/preline/preline.js",
  ],
  safelist: [
    "min-w-[6rem]",
    "ring-offset-white",
    "ring-blue-500",
    "shadow-xl",
    "max-h-48",
    "max-h-56",
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}

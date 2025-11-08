import postcssImport from "postcss-import";
import tailwindcss from "tailwindcss";
import autoprefixer from "autoprefixer";
import cssnano from "cssnano";

const isProduction = process.env.NODE_ENV === "production";

export default {
  plugins: [
    postcssImport(),
    tailwindcss(),
    autoprefixer(),
    ...(isProduction ? [cssnano({ preset: "default" })] : []),
  ],
};

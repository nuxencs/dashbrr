import { lerpColors } from "tailwind-lerp-colors";
import forms from "@tailwindcss/forms";

const extendedColors = lerpColors();

/** @type {import('tailwindcss').Config} */
export default {
  content: ["./src/**/*.{tsx,ts,html,css}"],
  safelist: [
    "col-span-1",
    "col-span-2",
    "col-span-3",
    "col-span-4",
    "col-span-5",
    "col-span-6",
    "col-span-7",
    "col-span-8",
    "col-span-9",
    "col-span-10",
    "col-span-11",
    "col-span-12",
  ],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        ...extendedColors,
        gray: {
          ...extendedColors.zinc,
          815: "#232427",
        },
      },
      margin: {
        2.5: "0.625rem",
      },
      textShadow: {
        DEFAULT: "0 2px 4px var(--tw-shadow-color)",
      },
      boxShadow: {
        table: "rgba(0, 0, 0, 0.1) 0px 4px 16px 0px",
      },
      animation: {
        fadeIn: "fadeIn 0.5s ease-in-out",
        bounce: "bounce 1s infinite",
        shimmer: "shimmer 2s infinite linear",
      },
      keyframes: {
        fadeIn: {
          "0%": { opacity: "0", transform: "translateY(10px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        bounce: {
          "0%, 100%": {
            transform: "translateY(-25%)",
            animationTimingFunction: "cubic-bezier(0.8, 0, 1, 1)",
          },
          "50%": {
            transform: "translateY(0)",
            animationTimingFunction: "cubic-bezier(0, 0, 0.2, 1)",
          },
        },
        shimmer: {
          "100%": { transform: "translateX(100%)" },
        },
      },
    },
  },
  variants: {
    extend: {},
  },
  plugins: [forms],
};

/** @type {import('tailwindcss').Config} */
module.exports = {
  // Scan every template and any JS that contains literal class strings.
  content: ["./templates/**/*.html", "./static/js/**/*.js"],
  // Class-based dark mode so the nav toggle can flip themes (a `.dark` class is
  // added to <html> on boot in header.html).
  darkMode: "class",
  theme: {
    extend: {
      fontFamily: {
        sans: ["Geist", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ['"Geist Mono"', "ui-monospace", "SFMono-Regular", "monospace"],
      },
    },
  },
  plugins: [],
};

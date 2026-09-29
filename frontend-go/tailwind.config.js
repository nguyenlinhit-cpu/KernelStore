/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./internal/**/*.templ", "./internal/**/*.go", "./static/js/app.js"],
  // Class chỉ được bật/tắt bằng JS/điều kiện có thể không bị quét thấy,
  // nên safelist để dòng lỗi luôn ẩn được.
  safelist: ["invisible"],
  theme: {
    extend: {
      colors: {
        bg: {
          primary: "#05070a",
          secondary: "#0b0f14",
          tertiary: "#11161d",
        },
        fg: {
          primary: "#e6f0ea",
          secondary: "#00d4aa",
          muted: "#7d8a83",
          neon: "#00ff5f",
        },
        accent: {
          DEFAULT: "#00ff5f",
          soft: "#00d4aa",
        },
      },
      fontFamily: {
        mono: [
          "JetBrains Mono",
          "Fira Code",
          "ui-monospace",
          "SFMono-Regular",
          "Menlo",
          "monospace",
        ],
      },
      borderRadius: {
        DEFAULT: "8px",
        lg: "12px",
        xl: "16px",
      },
      boxShadow: {
        glow: "0 0 0 1px rgba(0,255,95,0.25), 0 0 24px rgba(0,255,95,0.12)",
        "glow-lg": "0 0 0 1px rgba(0,255,95,0.35), 0 12px 48px rgba(0,255,95,0.18)",
        panel: "0 16px 48px rgba(0,0,0,0.55)",
      },
      backgroundImage: {
        "grid-terminal":
          "linear-gradient(rgba(0,255,95,0.035) 1px, transparent 1px), linear-gradient(90deg, rgba(0,255,95,0.035) 1px, transparent 1px)",
        "accent-sheen":
          "linear-gradient(135deg, #00ff5f 0%, #00d4aa 60%, #00ccff 100%)",
      },
      keyframes: {
        "fade-up": {
          "0%": { opacity: "0", transform: "translateY(14px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        "fade-in": {
          "0%": { opacity: "0" },
          "100%": { opacity: "1" },
        },
        float: {
          "0%,100%": { transform: "translateY(0)" },
          "50%": { transform: "translateY(-8px)" },
        },
        shimmer: {
          "0%": { backgroundPosition: "-200% 0" },
          "100%": { backgroundPosition: "200% 0" },
        },
        "pulse-glow": {
          "0%,100%": { opacity: "0.55" },
          "50%": { opacity: "1" },
        },
      },
      animation: {
        "fade-up": "fade-up 0.5s cubic-bezier(0.22,1,0.36,1) both",
        "fade-in": "fade-in 0.6s ease both",
        float: "float 6s ease-in-out infinite",
        shimmer: "shimmer 2.4s linear infinite",
        "pulse-glow": "pulse-glow 2.4s ease-in-out infinite",
      },
    },
  },
  plugins: [],
};

const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const src = path.join(root, "index.html");
const distDir = path.join(root, "dist");
const dest = path.join(distDir, "index.html");

fs.mkdirSync(distDir, { recursive: true });
fs.copyFileSync(src, dest);
console.log(`copied ${src} -> ${dest}`);


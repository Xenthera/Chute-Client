const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const src = path.join(root, "index.html");
const stylesSrc = path.join(root, "styles.css");
const distDir = path.join(root, "dist");
const dest = path.join(distDir, "index.html");
const assetsSrc = path.join(root, "assets");
const assetsDest = path.join(distDir, "assets");

fs.mkdirSync(distDir, { recursive: true });
fs.copyFileSync(src, dest);
console.log(`copied ${src} -> ${dest}`);
if (fs.existsSync(stylesSrc)) {
  const stylesDest = path.join(distDir, "styles.css");
  fs.copyFileSync(stylesSrc, stylesDest);
  console.log(`copied ${stylesSrc} -> ${stylesDest}`);
}

if (fs.existsSync(assetsSrc)) {
  fs.mkdirSync(assetsDest, { recursive: true });
  for (const entry of fs.readdirSync(assetsSrc)) {
    const from = path.join(assetsSrc, entry);
    const to = path.join(assetsDest, entry);
    if (fs.statSync(from).isFile()) {
      fs.copyFileSync(from, to);
    }
  }
  console.log(`copied ${assetsSrc} -> ${assetsDest}`);
}


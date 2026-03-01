import { chromium } from "playwright";
import { execSync, spawn } from "node:child_process";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, "../../..");
const PORT = 16275;
const BASE = `http://localhost:${PORT}`;
const IMAGES_DIR = resolve(ROOT, "images");
const TESTDATA = resolve(ROOT, "testdata");

async function waitForServer(maxRetries = 30) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const res = await fetch(`${BASE}/_/api/groups`);
      if (res.ok) return;
    } catch {}
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error("Server did not start");
}

async function addFile(path, group = "default") {
  const res = await fetch(`${BASE}/_/api/files`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path, group }),
  });
  if (!res.ok) throw new Error(`Failed to add file: ${path}`);
  return res.json();
}

async function waitForRendering(page) {
  // Wait for Shiki-highlighted code blocks (have .shiki class)
  await page
    .locator(".shiki")
    .first()
    .waitFor({ timeout: 15000 })
    .catch(() => {});
  // Wait for Mermaid SVGs
  await page
    .locator("svg[id^='mermaid']")
    .first()
    .waitFor({ timeout: 15000 })
    .catch(() => {});
  // Extra settle time
  await page.waitForTimeout(1000);
}

async function main() {
  // Build the Go binary
  console.log("Building mo...");
  execSync("make build", { cwd: ROOT, stdio: "inherit" });

  // Start server with an initial file
  console.log("Starting server on port", PORT);
  const firstFile = resolve(TESTDATA, "basic.md");
  const server = spawn(resolve(ROOT, "mo"), [firstFile, "-p", String(PORT)], {
    cwd: ROOT,
    stdio: "ignore",
  });

  try {
    await waitForServer();
    console.log("Server ready");

    const browser = await chromium.launch({ channel: "chrome" });
    const context = await browser.newContext({
      viewport: { width: 1200, height: 800 },
      deviceScaleFactor: 2,
      colorScheme: "dark",
    });

    try {
      // === Screenshot 1: multiple-files.png ===
      console.log("Taking multiple-files screenshot...");

      // Add files to default group
      await addFile(resolve(TESTDATA, "gfm.md"));
      await addFile(resolve(TESTDATA, "code-blocks.md"));

      const page1 = await context.newPage();
      await page1.addInitScript(() => {
        localStorage.setItem("mo-theme", "dark");
      });
      await page1.goto(BASE);
      await page1.waitForLoadState("load");

      // Click on code-blocks.md in sidebar
      await page1.locator("text=code-blocks.md").click();
      await page1.waitForTimeout(500);
      await waitForRendering(page1);

      await page1.screenshot({
        path: resolve(IMAGES_DIR, "multiple-files.png"),
      });
      console.log("Saved multiple-files.png");
      await page1.close();

      // === Screenshot 2: groups.png ===
      console.log("Taking groups screenshot...");

      // Add files to "design" group
      await addFile(resolve(TESTDATA, "mermaid-sequence.md"), "design");
      await addFile(resolve(TESTDATA, "mermaid-flowchart.md"), "design");
      await addFile(resolve(TESTDATA, "mermaid-mixed.md"), "design");

      // Add files to "plan" group
      await addFile(resolve(TESTDATA, "lists.md"), "plan");

      const page2 = await context.newPage();
      // Set dark theme before navigating so Mermaid initializes with dark theme
      await page2.addInitScript(() => {
        localStorage.setItem("mo-theme", "dark");
      });
      await page2.goto(`${BASE}/design`);
      await page2.waitForLoadState("load");

      // Click on mermaid-mixed.md in sidebar
      await page2.locator("text=mermaid-mixed.md").click();
      await page2.waitForTimeout(500);
      await waitForRendering(page2);

      // Open the group dropdown
      await page2.locator("button:has-text('design')").click();
      await page2.waitForTimeout(300);

      await page2.screenshot({
        path: resolve(IMAGES_DIR, "groups.png"),
      });
      console.log("Saved groups.png");
      await page2.close();
    } finally {
      await browser.close();
    }
  } finally {
    server.kill();
    // Kill any remaining mo process on the port
    try {
      execSync(`lsof -i :${PORT} -t | xargs kill 2>/dev/null`, {
        stdio: "ignore",
      });
    } catch {}
    console.log("Done");
  }
}

main().catch((err) => {
  console.error(err);
  try {
    execSync(`lsof -i :${PORT} -t | xargs kill 2>/dev/null`, {
      stdio: "ignore",
    });
  } catch {}
  process.exit(1);
});

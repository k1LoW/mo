const markdownExtensions = new Set(["md", "mdx", "markdown", "mdown", "mkdn", "mkd"]);

export function isMarkdownFile(fileName: string): boolean {
  const ext = fileName.split(".").pop()?.toLowerCase() ?? "";
  return markdownExtensions.has(ext);
}

// Map file extension to Shiki language identifier.
// Returns "text" for unknown extensions.
const extToLang: Record<string, string> = {
  ts: "typescript",
  tsx: "tsx",
  js: "javascript",
  jsx: "jsx",
  json: "json",
  jsonc: "jsonc",
  yaml: "yaml",
  yml: "yaml",
  toml: "toml",
  xml: "xml",
  html: "html",
  css: "css",
  scss: "scss",
  less: "less",
  go: "go",
  rs: "rust",
  py: "python",
  rb: "ruby",
  java: "java",
  kt: "kotlin",
  kts: "kotlin",
  swift: "swift",
  c: "c",
  h: "c",
  cpp: "cpp",
  hpp: "cpp",
  cc: "cpp",
  cs: "csharp",
  sh: "bash",
  bash: "bash",
  zsh: "bash",
  fish: "fish",
  ps1: "powershell",
  sql: "sql",
  graphql: "graphql",
  gql: "graphql",
  dockerfile: "dockerfile",
  tf: "hcl",
  hcl: "hcl",
  lua: "lua",
  php: "php",
  r: "r",
  scala: "scala",
  zig: "zig",
  elm: "elm",
  ex: "elixir",
  exs: "elixir",
  erl: "erlang",
  hs: "haskell",
  clj: "clojure",
  vim: "viml",
  diff: "diff",
  ini: "ini",
  conf: "ini",
  cfg: "ini",
  env: "ini",
  txt: "text",
  log: "text",
  csv: "csv",
  svg: "xml",
};

export function detectLanguage(fileName: string): string {
  const lower = fileName.toLowerCase();
  // Handle dotfiles and special filenames
  const basename = lower.split("/").pop() ?? lower;
  if (basename === "dockerfile" || basename.startsWith("dockerfile.")) return "dockerfile";
  if (basename === "makefile" || basename === "gnumakefile") return "makefile";

  const ext = basename.split(".").pop() ?? "";
  return extToLang[ext] ?? "text";
}

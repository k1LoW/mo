import { useEffect, useState, useCallback, useMemo } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import { codeToHtml } from "shiki";
import mermaid from "mermaid";
import { fetchFileContent, openRelativeFile } from "../hooks/useApi";
import type { Components } from "react-markdown";
import "github-markdown-css/github-markdown.css";

interface MarkdownViewerProps {
  fileId: number;
  revision: number;
  onFileOpened: (fileId: number) => void;
}

let mermaidInitialized = false;

function getMermaidTheme(): "dark" | "default" {
  return document.documentElement.getAttribute("data-theme") === "dark"
    ? "dark"
    : "default";
}

function ensureMermaidInit() {
  if (!mermaidInitialized) {
    mermaid.initialize({ startOnLoad: false, theme: getMermaidTheme() });
    mermaidInitialized = true;
  }
}

let mermaidCounter = 0;

function cleanupMermaidErrors() {
  document.querySelectorAll("[id^='dmermaid-']").forEach((el) => el.remove());
}

async function renderMermaid(code: string): Promise<string> {
  const id = `mermaid-${++mermaidCounter}`;
  const container = document.createElement("div");
  container.style.position = "absolute";
  container.style.left = "-9999px";
  container.style.top = "-9999px";
  document.body.appendChild(container);
  try {
    const { svg } = await mermaid.render(id, code, container);
    return svg;
  } finally {
    container.remove();
    cleanupMermaidErrors();
  }
}

function MermaidBlock({ code }: { code: string }) {
  const [svg, setSvg] = useState("");

  useEffect(() => {
    let cancelled = false;

    ensureMermaidInit();
    mermaid.initialize({ startOnLoad: false, theme: getMermaidTheme() });

    renderMermaid(code)
      .then((renderedSvg) => {
        if (!cancelled) setSvg(renderedSvg);
      })
      .catch(() => {
        if (!cancelled) setSvg("");
      });

    return () => {
      cancelled = true;
    };
  }, [code]);

  // Re-render on theme change
  useEffect(() => {
    const observer = new MutationObserver(() => {
      mermaid.initialize({ startOnLoad: false, theme: getMermaidTheme() });
      renderMermaid(code)
        .then((renderedSvg) => setSvg(renderedSvg))
        .catch(() => {});
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => observer.disconnect();
  }, [code]);

  if (svg) {
    return <div dangerouslySetInnerHTML={{ __html: svg }} />;
  }
  return (
    <pre>
      <code>{code}</code>
    </pre>
  );
}

function CodeBlock({ language, code }: { language: string; code: string }) {
  const [html, setHtml] = useState("");

  useEffect(() => {
    let cancelled = false;
    codeToHtml(code, { lang: language, theme: "github-dark" })
      .then((result) => {
        if (!cancelled) setHtml(result);
      })
      .catch(() => {
        // Fallback: if language not supported, try plaintext
        if (!cancelled) {
          codeToHtml(code, { lang: "text", theme: "github-dark" })
            .then((result) => {
              if (!cancelled) setHtml(result);
            })
            .catch(() => {});
        }
      });
    return () => {
      cancelled = true;
    };
  }, [code, language]);

  if (html) {
    return <div dangerouslySetInnerHTML={{ __html: html }} />;
  }
  return (
    <pre>
      <code>{code}</code>
    </pre>
  );
}

export function MarkdownViewer({ fileId, revision, onFileOpened }: MarkdownViewerProps) {
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetchFileContent(fileId)
      .then((data) => {
        if (!cancelled) {
          setContent(data.content);
          setLoading(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setContent("Failed to load file.");
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [fileId, revision]);

  const handleLinkClick = useCallback(
    async (e: React.MouseEvent<HTMLAnchorElement>, href: string) => {
      if (!href.endsWith(".md")) return;
      if (href.startsWith("http://") || href.startsWith("https://")) return;
      e.preventDefault();
      try {
        const entry = await openRelativeFile(fileId, href);
        onFileOpened(entry.id);
      } catch {
        // fallback: do nothing
      }
    },
    [fileId, onFileOpened],
  );

  const components: Components = useMemo(
    () => ({
      pre: ({ children }) => <>{children}</>,
      code: ({ className, children, ...props }) => {
        const match = /language-(\w+)/.exec(className || "");
        const code = String(children).replace(/\n$/, "");
        if (match) {
          if (match[1] === "mermaid") {
            return <MermaidBlock code={code} />;
          }
          return <CodeBlock language={match[1]} code={code} />;
        }
        return (
          <code className={className} {...props}>
            {children}
          </code>
        );
      },
      img: ({ src, alt, ...props }) => {
        const resolvedSrc =
          src && !src.startsWith("http://") && !src.startsWith("https://")
            ? `/_/api/files/${fileId}/raw/${src}`
            : src;
        return <img src={resolvedSrc} alt={alt} {...props} />;
      },
      a: ({ href, children, ...props }) => {
        if (!href || href.startsWith("http://") || href.startsWith("https://") || href.startsWith("#")) {
          return (
            <a href={href} target={href?.startsWith("#") ? undefined : "_blank"} rel={href?.startsWith("#") ? undefined : "noopener noreferrer"} {...props}>
              {children}
            </a>
          );
        }
        // Strip anchor from href for file detection
        const hrefPath = href.split("#")[0];
        if (hrefPath.endsWith(".md")) {
          return (
            <a
              href={href}
              onClick={(e) => handleLinkClick(e, hrefPath)}
              {...props}
            >
              {children}
            </a>
          );
        }
        // Only rewrite to raw API if the path looks like a file (has an extension)
        const basename = hrefPath.split("/").pop() || "";
        if (basename.includes(".")) {
          return (
            <a href={`/_/api/files/${fileId}/raw/${href}`} {...props}>
              {children}
            </a>
          );
        }
        // Directories or extensionless paths: leave as-is
        return (
          <a href={href} {...props}>
            {children}
          </a>
        );
      },
    }),
    [fileId, handleLinkClick],
  );

  if (loading) {
    return <div className="flex items-center justify-center h-50 text-gh-text-secondary text-sm">Loading...</div>;
  }

  return (
    <article className="markdown-body">
      <Markdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeRaw]} components={components}>
        {content}
      </Markdown>
    </article>
  );
}

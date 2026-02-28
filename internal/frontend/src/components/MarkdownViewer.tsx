import { useEffect, useState, useCallback, useMemo } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import { rehypeGithubAlerts } from "rehype-github-alerts";
import { codeToHtml } from "shiki";
import mermaid from "mermaid";
import { fetchFileContent, openRelativeFile } from "../hooks/useApi";
import { RawToggle } from "./RawToggle";
import { resolveLink, resolveImageSrc, extractLanguage } from "../utils/resolve";
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
let mermaidQueue: Promise<void> = Promise.resolve();

function cleanupMermaidErrors() {
  document.querySelectorAll("[id^='dmermaid-']").forEach((el) => el.remove());
}

async function renderMermaid(code: string): Promise<string> {
  let resolve: (svg: string) => void;
  let reject: (err: unknown) => void;
  const result = new Promise<string>((res, rej) => {
    resolve = res;
    reject = rej;
  });

  mermaidQueue = mermaidQueue.then(async () => {
    const id = `mermaid-${++mermaidCounter}`;
    const container = document.createElement("div");
    container.style.position = "absolute";
    container.style.left = "-9999px";
    container.style.top = "-9999px";
    document.body.appendChild(container);
    try {
      const { svg } = await mermaid.render(id, code, container);
      resolve!(svg);
    } catch (err) {
      reject!(err);
    } finally {
      container.remove();
      cleanupMermaidErrors();
    }
  });

  return result;
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

function RawView({ content }: { content: string }) {
  const [html, setHtml] = useState("");

  useEffect(() => {
    let cancelled = false;
    codeToHtml(content, { lang: "markdown", theme: "github-dark" })
      .then((result) => {
        if (!cancelled) setHtml(result);
      })
      .catch(() => {
        if (!cancelled) {
          codeToHtml(content, { lang: "text", theme: "github-dark" })
            .then((result) => {
              if (!cancelled) setHtml(result);
            })
            .catch(() => {});
        }
      });
    return () => {
      cancelled = true;
    };
  }, [content]);

  if (html) {
    return <div className="[&_pre]:!rounded-none" dangerouslySetInnerHTML={{ __html: html }} />;
  }
  return (
    <pre>
      <code>{content}</code>
    </pre>
  );
}

export function MarkdownViewer({ fileId, revision, onFileOpened }: MarkdownViewerProps) {
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(true);
  const [isRawView, setIsRawView] = useState(false);

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
        const language = extractLanguage(className);
        const code = String(children).replace(/\n$/, "");
        if (language) {
          if (language === "mermaid") {
            return <MermaidBlock code={code} />;
          }
          return <CodeBlock language={language} code={code} />;
        }
        return (
          <code className={className} {...props}>
            {children}
          </code>
        );
      },
      img: ({ src, alt, ...props }) => {
        return <img src={resolveImageSrc(src, fileId)} alt={alt} {...props} />;
      },
      a: ({ href, children, ...props }) => {
        const resolved = resolveLink(href, fileId);
        switch (resolved.type) {
          case "external":
            return (
              <a href={href} target="_blank" rel="noopener noreferrer" {...props}>
                {children}
              </a>
            );
          case "hash":
            return (
              <a href={href} {...props}>
                {children}
              </a>
            );
          case "markdown":
            return (
              <a
                href={href}
                onClick={(e) => handleLinkClick(e, resolved.hrefPath)}
                {...props}
              >
                {children}
              </a>
            );
          case "file":
            return (
              <a href={resolved.rawUrl} {...props}>
                {children}
              </a>
            );
          case "passthrough":
            return (
              <a href={href} {...props}>
                {children}
              </a>
            );
        }
      },
    }),
    [fileId, handleLinkClick],
  );

  if (loading) {
    return <div className="flex items-center justify-center h-50 text-gh-text-secondary text-sm">Loading...</div>;
  }

  return (
    <div className="flex items-start gap-2">
      <article className="markdown-body min-w-0 flex-1">
        {isRawView ? (
          <RawView content={content} />
        ) : (
          <Markdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeRaw, rehypeGithubAlerts]} components={components}>
            {content}
          </Markdown>
        )}
      </article>
      <div className="shrink-0 flex flex-col gap-2 -mr-4">
        <RawToggle isRaw={isRawView} onToggle={() => setIsRawView((v) => !v)} />
      </div>
    </div>
  );
}

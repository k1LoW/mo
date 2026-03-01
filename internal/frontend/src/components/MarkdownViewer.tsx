import { useEffect, useRef, useState, useCallback, useMemo } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import rehypeSlug from "rehype-slug";
import { rehypeGithubAlerts } from "rehype-github-alerts";
import { codeToHtml } from "shiki";
import mermaid from "mermaid";
import { fetchFileContent, openRelativeFile } from "../hooks/useApi";
import { RawToggle } from "./RawToggle";
import { TocToggle } from "./TocToggle";
import { CopyButton } from "./CopyButton";
import { resolveLink, resolveImageSrc, extractLanguage } from "../utils/resolve";
import { extractText } from "../utils/extractText";
import type { TocHeading } from "./TocPanel";
import type { Components } from "react-markdown";
import "github-markdown-css/github-markdown.css";

interface MarkdownViewerProps {
  fileId: number;
  revision: number;
  onFileOpened: (fileId: number) => void;
  onHeadingsChange: (headings: TocHeading[]) => void;
  isTocOpen: boolean;
  onTocToggle: () => void;
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

function CodeBlockCopyButton({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
    } catch {
      // clipboard API may fail in insecure contexts
    }
  };

  return (
    <button
      className={`absolute right-2 top-2 flex items-center justify-center rounded-md p-1 cursor-pointer transition-all duration-150 border border-[#484f58] hover:border-[#8b949e] ${copied ? "opacity-100 text-[#8b949e] bg-[#2d333b]" : "opacity-0 group-hover:opacity-100 text-[#8b949e] bg-[#2d333b]"}`}
      onClick={handleCopy}
      title="Copy code"
    >
      {copied ? (
        <svg className="size-4" viewBox="0 0 16 16" fill="currentColor">
          <path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z" />
        </svg>
      ) : (
        <svg className="size-4" viewBox="0 0 16 16" fill="currentColor">
          <path d="M0 6.75C0 5.784.784 5 1.75 5h1.5a.75.75 0 0 1 0 1.5h-1.5a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-1.5a.75.75 0 0 1 1.5 0v1.5A1.75 1.75 0 0 1 9.25 16h-7.5A1.75 1.75 0 0 1 0 14.25ZM5 1.75C5 .784 5.784 0 6.75 0h7.5C15.216 0 16 .784 16 1.75v7.5A1.75 1.75 0 0 1 14.25 11h-7.5A1.75 1.75 0 0 1 5 9.25Zm1.75-.25a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-7.5a.25.25 0 0 0-.25-.25Z" />
        </svg>
      )}
    </button>
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
    return (
      <div className="relative group">
        <div dangerouslySetInnerHTML={{ __html: html }} />
        <CodeBlockCopyButton code={code} />
      </div>
    );
  }
  return (
    <div className="relative group">
      <pre>
        <code>{code}</code>
      </pre>
      <CodeBlockCopyButton code={code} />
    </div>
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

export function MarkdownViewer({ fileId, revision, onFileOpened, onHeadingsChange, isTocOpen, onTocToggle }: MarkdownViewerProps) {
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(true);
  const [isRawView, setIsRawView] = useState(false);
  const headingsRef = useRef<TocHeading[]>([]);

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

  // Reset heading collector before each render
  headingsRef.current = [];

  const createHeading = useCallback(
    (level: number) =>
      ({ id, children, ...props }: React.JSX.IntrinsicElements["h1"]) => {
        const text = extractText(children);
        if (id) {
          headingsRef.current.push({ id: String(id), text, level });
        }
        const Tag = `h${level}` as "h1" | "h2" | "h3" | "h4" | "h5" | "h6";
        return <Tag id={id} {...props}>{children}</Tag>;
      },
    [],
  );

  const components: Components = useMemo(
    () => ({
      h1: createHeading(1),
      h2: createHeading(2),
      h3: createHeading(3),
      h4: createHeading(4),
      h5: createHeading(5),
      h6: createHeading(6),
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
    [fileId, handleLinkClick, createHeading],
  );

  const renderedContent = useMemo(() => {
    if (isRawView) {
      return <RawView content={content} />;
    }
    return (
      <Markdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeRaw, rehypeGithubAlerts, rehypeSlug]} components={components}>
        {content}
      </Markdown>
    );
  }, [content, isRawView, components]);

  const prevHeadingsKey = useRef("");
  useEffect(() => {
    const newHeadings = isRawView ? [] : [...headingsRef.current];
    const key = newHeadings.map((h) => `${h.id}:${h.level}`).join(",");
    if (key !== prevHeadingsKey.current) {
      prevHeadingsKey.current = key;
      onHeadingsChange(newHeadings);
    }
  }, [content, isRawView, onHeadingsChange, renderedContent]);

  if (loading) {
    return <div className="flex items-center justify-center h-50 text-gh-text-secondary text-sm">Loading...</div>;
  }

  return (
    <div className="flex items-start gap-2">
      <article className="markdown-body min-w-0 flex-1">
        {renderedContent}
      </article>
      <div className="shrink-0 flex flex-col gap-2 -mr-4 -mt-4">
        <TocToggle isTocOpen={isTocOpen} onToggle={onTocToggle} />
        <RawToggle isRaw={isRawView} onToggle={() => setIsRawView((v) => !v)} />
        <CopyButton content={content} />
      </div>
    </div>
  );
}

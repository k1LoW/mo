import { useEffect, useLayoutEffect, useRef, useState, useCallback, useMemo } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import rehypeSlug from "rehype-slug";
import rehypeKatex from "rehype-katex";
import { rehypeGithubAlerts } from "rehype-github-alerts";
import "katex/dist/katex.min.css";
import { codeToHtml } from "shiki";
import mermaid from "mermaid";
import { fetchFileContent, openRelativeFile } from "../hooks/useApi";
import { escapeRegExp } from "../utils/regex";
import { RawToggle } from "./RawToggle";
import { TocToggle } from "./TocToggle";
import { CopyButton } from "./CopyButton";
import { CloseFileButton } from "./CloseFileButton";
import { resolveLink, resolveImageSrc, extractLanguage } from "../utils/resolve";
import { parseFrontmatter } from "../utils/frontmatter";
import { stripMdxSyntax } from "../utils/mdx";
import { isMarkdownFile, detectLanguage } from "../utils/filetype";
import type { ZoomContent } from "./ZoomModal";
import type { TocHeading } from "./TocPanel";
import type { Components } from "react-markdown";
import "github-markdown-css/github-markdown.css";

// Strip the `user-content-` prefix that remark-gfm bakes into footnote IDs,
// so rehype-sanitize can re-add it exactly once (avoiding double-prefixed IDs).
function rehypeStripClobberPrefix() {
  const FOOTNOTE_ID_PATTERN = /^user-content-(fn-|fnref-|footnote-label$)/;
  const PREFIX = "user-content-";
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function walk(node: any) {
    if (node.properties) {
      const props = node.properties;
      if (typeof props.id === "string" && FOOTNOTE_ID_PATTERN.test(props.id)) {
        props.id = props.id.slice(PREFIX.length);
      }
    }
    if (node.children) {
      for (const child of node.children) {
        if (child.type === "element") walk(child);
      }
    }
  }
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return (tree: any) => {
    walk(tree);
  };
}

// Extend default GitHub-compatible schema to allow style/align attributes used in raw HTML
const sanitizeSchema = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    span: [...(defaultSchema.attributes?.["span"] || []), "style"],
    div: [...(defaultSchema.attributes?.["div"] || []), "style", "align"],
  },
};

interface MarkdownViewerProps {
  fileId: string;
  fileName: string;
  activeGroup: string;
  revision: number;
  onFileOpened: (fileId: string) => void;
  onHeadingsChange: (headings: TocHeading[]) => void;
  onContentRendered?: () => void;
  isTocOpen: boolean;
  onTocToggle: () => void;
  onRemoveFile: () => void;
  uploaded?: boolean;
  isWide: boolean;
  onZoom?: (content: ZoomContent) => void;
  scrollToHeading?: string | null;
  onScrolledToHeading?: () => void;
  searchQuery?: string | null;
}

interface SearchHitMarker {
  top: number;
  height: number;
}

const SEARCH_HIT_COLUMN_OFFSET = -24;

function collectSearchHitMarkers(root: HTMLElement, query: string): SearchHitMarker[] {
  const trimmed = query.trim();
  if (!trimmed) {
    return [];
  }

  const pattern = new RegExp(escapeRegExp(trimmed), "gi");
  const articleRect = root.getBoundingClientRect();
  const markers = new Map<string, SearchHitMarker>();
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      const parent = node.parentElement;
      if (
        parent == null ||
        parent.closest("script, style, .frontmatter-block") != null ||
        node.textContent == null ||
        node.textContent.trim() === ""
      ) {
        return NodeFilter.FILTER_REJECT;
      }
      pattern.lastIndex = 0;
      return pattern.test(node.textContent) ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT;
    },
  });

  let current = walker.nextNode();
  while (current != null) {
    if (current instanceof Text) {
      const text = current.textContent ?? "";
      pattern.lastIndex = 0;
      for (const match of text.matchAll(pattern)) {
        const start = match.index ?? 0;
        const end = start + match[0].length;
        const range = document.createRange();
        range.setStart(current, start);
        range.setEnd(current, end);
        const [rect] = Array.from(range.getClientRects());
        if (rect != null && rect.height > 0 && rect.width > 0) {
          const top = rect.top - articleRect.top;
          const height = rect.height;
          const key = `${Math.round(top)}:${Math.round(height)}`;
          markers.set(key, {
            top,
            height,
          });
        }
      }
    }
    current = walker.nextNode();
  }

  return [...markers.values()].sort((a, b) => a.top - b.top);
}

function getMermaidTheme(): "dark" | "default" {
  return document.documentElement.getAttribute("data-theme") === "dark" ? "dark" : "default";
}

let mermaidCounter = 0;
let mermaidQueue: Promise<void> = Promise.resolve();

function cleanupMermaidErrors() {
  document.querySelectorAll("[id^='dmermaid-']").forEach((el) => el.remove());
}

async function renderMermaid(code: string, width?: number): Promise<string> {
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
    container.style.width = `${width && width > 0 ? width : 800}px`;
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

export function MermaidBlock({
  code,
  onZoom,
}: {
  code: string;
  onZoom?: (content: ZoomContent) => void;
}) {
  const [svg, setSvg] = useState("");
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;

    const doRender = () => {
      const width = containerRef.current?.offsetWidth;
      mermaid.initialize({ startOnLoad: false, theme: getMermaidTheme() });
      renderMermaid(code, width)
        .then((renderedSvg) => {
          if (!cancelled) setSvg(renderedSvg);
        })
        .catch(() => {
          if (!cancelled) setSvg("");
        });
    };

    doRender();

    // Re-render on theme change
    const observer = new MutationObserver(() => doRender());
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    return () => {
      cancelled = true;
      observer.disconnect();
    };
  }, [code]);

  if (svg) {
    return (
      <div ref={containerRef} className="relative group">
        <div className="overflow-x-auto" dangerouslySetInnerHTML={{ __html: svg }} />
        {onZoom && <ZoomButton onClick={() => onZoom({ type: "svg", svg })} position="right-18" />}
        <MermaidImageCopyButton svg={svg} />
        <CodeBlockCopyButton code={code} themed />
      </div>
    );
  }
  return (
    <div ref={containerRef} className="relative group">
      <pre>
        <code>{code}</code>
      </pre>
      <CodeBlockCopyButton code={code} />
    </div>
  );
}

function MermaidImageCopyButton({ svg }: { svg: string }) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  const handleCopy = async () => {
    try {
      const blob = await svgToPngBlob(svg);
      await navigator.clipboard.write([new ClipboardItem({ "image/png": blob })]);
      setCopied(true);
    } catch {
      // clipboard API may fail in insecure contexts
    }
  };

  return (
    <button
      className={`absolute right-10 top-2 flex items-center justify-center rounded-md p-1 cursor-pointer transition-all duration-150 border ${themedButtonStyle} ${copied ? "opacity-100" : "opacity-0 group-hover:opacity-100"}`}
      onClick={handleCopy}
      title="Copy image"
    >
      {copied ? (
        <svg className="size-4" viewBox="0 0 16 16" fill="currentColor">
          <path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z" />
        </svg>
      ) : (
        <svg className="size-4" viewBox="0 0 16 16" fill="currentColor">
          <path d="M16 13.25A1.75 1.75 0 0 1 14.25 15H1.75A1.75 1.75 0 0 1 0 13.25V2.75C0 1.784.784 1 1.75 1h12.5c.966 0 1.75.784 1.75 1.75ZM1.75 2.5a.25.25 0 0 0-.25.25v10.5c0 .138.112.25.25.25h12.5a.25.25 0 0 0 .25-.25V2.75a.25.25 0 0 0-.25-.25Z" />
          <path
            d="M0.5 12.75 4.5 5.5 7.5 9 9.5 6.5 15.5 12.75"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinejoin="round"
          />
        </svg>
      )}
    </button>
  );
}

function svgToPngBlob(svgString: string): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const parser = new DOMParser();
    const doc = parser.parseFromString(svgString, "image/svg+xml");
    const svgEl = doc.documentElement;

    // Ensure xmlns is present for standalone SVG rendering
    if (!svgEl.getAttribute("xmlns")) {
      svgEl.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    }

    // Extract dimensions from the SVG element
    const widthAttr = svgEl.getAttribute("width");
    const heightAttr = svgEl.getAttribute("height");
    const viewBox = svgEl.getAttribute("viewBox");

    let width = 0;
    let height = 0;

    if (widthAttr && heightAttr) {
      width = parseFloat(widthAttr);
      height = parseFloat(heightAttr);
    } else if (viewBox) {
      const parts = viewBox.split(/[\s,]+/);
      width = parseFloat(parts[2]);
      height = parseFloat(parts[3]);
    }

    if (!width || !height) {
      reject(new Error("Cannot determine SVG dimensions"));
      return;
    }

    // Scale up for high-DPI displays
    const scale = 4;
    const serializer = new XMLSerializer();
    const svgData = serializer.serializeToString(svgEl);
    const dataUrl = "data:image/svg+xml;charset=utf-8," + encodeURIComponent(svgData);

    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement("canvas");
      canvas.width = width * scale;
      canvas.height = height * scale;
      const ctx = canvas.getContext("2d");
      if (!ctx) {
        reject(new Error("Failed to get canvas context"));
        return;
      }
      ctx.scale(scale, scale);
      ctx.drawImage(img, 0, 0, width, height);
      canvas.toBlob((blob) => {
        if (blob) {
          resolve(blob);
        } else {
          reject(new Error("Failed to create PNG blob"));
        }
      }, "image/png");
    };
    img.onerror = () => {
      reject(new Error("Failed to load SVG image"));
    };
    img.src = dataUrl;
  });
}

function ZoomButton({
  onClick,
  position = "right-2",
  groupClass = "group-hover:opacity-100",
}: {
  onClick: () => void;
  position?: string;
  groupClass?: string;
}) {
  return (
    <button
      className={`absolute ${position} top-2 flex items-center justify-center rounded-md p-1 cursor-pointer transition-all duration-150 border ${themedButtonStyle} opacity-0 ${groupClass}`}
      onClick={onClick}
      title="Zoom"
    >
      {/* Placeholder icon — will be replaced */}
      <svg
        className="size-4"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
      >
        <circle cx="7" cy="7" r="4.5" />
        <line x1="10.5" y1="10.5" x2="14" y2="14" strokeLinecap="round" />
        <line x1="5" y1="7" x2="9" y2="7" strokeLinecap="round" />
        <line x1="7" y1="5" x2="7" y2="9" strokeLinecap="round" />
      </svg>
    </button>
  );
}

const darkButtonStyle = "border-[#484f58] hover:border-[#8b949e] text-[#8b949e] bg-[#2d333b]";
const themedButtonStyle =
  "border-gh-border hover:border-gh-text-secondary text-gh-text-secondary bg-gh-bg-secondary";

function CodeBlockCopyButton({ code, themed = false }: { code: string; themed?: boolean }) {
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

  const colorStyle = themed ? themedButtonStyle : darkButtonStyle;

  return (
    <button
      className={`absolute right-2 top-2 flex items-center justify-center rounded-md p-1 cursor-pointer transition-all duration-150 border ${colorStyle} ${copied ? "opacity-100" : "opacity-0 group-hover:opacity-100"}`}
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

function FrontmatterBlock({ yaml }: { yaml: string }) {
  return (
    <details open className="mb-4">
      <summary className="cursor-pointer select-none text-gh-text-secondary text-sm font-medium py-1">
        Metadata
      </summary>
      <div className="mt-2">
        <CodeBlock language="yaml" code={yaml} />
      </div>
    </details>
  );
}

function HighlightedView({ content, language }: { content: string; language: string }) {
  const [html, setHtml] = useState("");

  useEffect(() => {
    let cancelled = false;
    setHtml("");
    codeToHtml(content, { lang: language, theme: "github-dark" })
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
  }, [content, language]);

  if (html) {
    return <div className="[&_pre]:!rounded-none" dangerouslySetInnerHTML={{ __html: html }} />;
  }
  return (
    <pre>
      <code>{content}</code>
    </pre>
  );
}

function RawView({ content }: { content: string }) {
  return <HighlightedView content={content} language="markdown" />;
}

export function MarkdownViewer({
  fileId,
  fileName,
  activeGroup,
  revision,
  onFileOpened,
  onHeadingsChange,
  onContentRendered,
  isTocOpen,
  onTocToggle,
  onRemoveFile,
  uploaded,
  isWide,
  onZoom,
  scrollToHeading,
  onScrolledToHeading,
  searchQuery,
}: MarkdownViewerProps) {
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(true);
  const [isRawView, setIsRawView] = useState(false);
  const [searchHitMarkers, setSearchHitMarkers] = useState<SearchHitMarker[]>([]);
  const articleRef = useRef<HTMLElement>(null);
  const [prevFetchKey, setPrevFetchKey] = useState({ fileId, revision });

  if (fileId !== prevFetchKey.fileId || revision !== prevFetchKey.revision) {
    setPrevFetchKey({ fileId, revision });
    setLoading(true);
  }

  useEffect(() => {
    let cancelled = false;
    fetchFileContent(activeGroup, fileId)
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
  }, [activeGroup, fileId, revision]);

  const handleLinkClick = useCallback(
    async (e: React.MouseEvent<HTMLAnchorElement>, href: string) => {
      e.preventDefault();
      try {
        const entry = await openRelativeFile(activeGroup, fileId, href);
        onFileOpened(entry.id);
      } catch {
        // fallback: do nothing
      }
    },
    [activeGroup, fileId, onFileOpened],
  );

  const components: Components = useMemo(
    () => ({
      pre: ({ children }) => <>{children}</>,
      code: ({ className, children, ...props }) => {
        const language = extractLanguage(className);
        const code = String(children).replace(/\n$/, "");
        const isBlock = String(children).endsWith("\n");
        if (language) {
          if (language === "mermaid") {
            return <MermaidBlock code={code} onZoom={onZoom} />;
          }
          return <CodeBlock language={language} code={code} />;
        }
        if (isBlock) {
          return <CodeBlock language="text" code={code} />;
        }
        return (
          <code className={className} {...props}>
            {children}
          </code>
        );
      },
      img: ({ src, alt, ...props }) => {
        const resolvedSrc = resolveImageSrc(src, activeGroup, fileId);
        if (onZoom && resolvedSrc) {
          return (
            <span className="relative inline-block group/img">
              <img src={resolvedSrc} alt={alt} {...props} />
              <ZoomButton
                onClick={() => onZoom({ type: "image", src: resolvedSrc, alt: alt ?? undefined })}
                position="right-1"
                groupClass="group-hover/img:opacity-100"
              />
            </span>
          );
        }
        return <img src={resolveImageSrc(src, activeGroup, fileId)} alt={alt} {...props} />;
      },
      a: ({ href, children, ...props }) => {
        const resolved = resolveLink(href, activeGroup, fileId);
        switch (resolved.type) {
          case "external":
            return (
              <a href={href} target="_blank" rel="noopener noreferrer" {...props}>
                {children}
              </a>
            );
          case "hash":
            return (
              <a
                href={href}
                onClick={(e) => {
                  if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
                  const id = href?.slice(1);
                  if (!id) return;
                  const target = document.getElementById(id);
                  if (target) {
                    e.preventDefault();
                    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
                    target.scrollIntoView({
                      behavior: reduced ? "auto" : "smooth",
                      block: "start",
                    });
                    history.pushState(null, "", href);
                  }
                }}
                {...props}
              >
                {children}
              </a>
            );
          case "markdown":
            return (
              <a href={href} onClick={(e) => handleLinkClick(e, resolved.hrefPath)} {...props}>
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
    [fileId, handleLinkClick, onZoom],
  );

  const isMarkdown = isMarkdownFile(fileName);
  const codeLanguage = isMarkdown ? null : detectLanguage(fileName);

  const parsed = useMemo(
    () => (isMarkdown && !isRawView ? parseFrontmatter(content) : null),
    [content, isRawView, isMarkdown],
  );

  const renderedContent = useMemo(() => {
    if (!isMarkdown) {
      return <HighlightedView content={content} language={codeLanguage!} />;
    }
    if (isRawView) {
      return <RawView content={content} />;
    }
    const base = parsed ? parsed.content : content;
    const md = fileName.toLowerCase().endsWith(".mdx") ? stripMdxSyntax(base) : base;
    return (
      <>
        {parsed && <FrontmatterBlock yaml={parsed.yaml} />}
        <Markdown
          remarkPlugins={[remarkGfm, remarkMath]}
          rehypePlugins={[
            rehypeRaw,
            rehypeStripClobberPrefix,
            [rehypeSanitize, sanitizeSchema],
            rehypeGithubAlerts,
            rehypeSlug,
            rehypeKatex,
          ]}
          components={components}
        >
          {md}
        </Markdown>
      </>
    );
  }, [content, isRawView, isMarkdown, codeLanguage, parsed, components, fileName]);

  const prevHeadingsKey = useRef("");
  useEffect(() => {
    const newHeadings: TocHeading[] = [];
    if (!isRawView && articleRef.current) {
      const els = articleRef.current.querySelectorAll("h1, h2, h3, h4, h5, h6");
      for (const el of els) {
        if (el.id) {
          newHeadings.push({
            id: el.id,
            text: el.textContent ?? "",
            level: parseInt(el.tagName.slice(1), 10),
          });
        }
      }
    }
    const key = newHeadings.map((h) => `${h.id}:${h.level}:${h.text}`).join(",");
    if (key !== prevHeadingsKey.current) {
      prevHeadingsKey.current = key;
      onHeadingsChange(newHeadings);
    }
  }, [isRawView, renderedContent, onHeadingsChange]);

  const onContentRenderedRef = useRef(onContentRendered);
  useLayoutEffect(() => {
    onContentRenderedRef.current = onContentRendered;
  });

  useLayoutEffect(() => {
    if (!loading) {
      onContentRenderedRef.current?.();
    }
  }, [loading, renderedContent]);

  useLayoutEffect(() => {
    if (loading || !scrollToHeading || !articleRef.current) {
      return;
    }

    const headings = articleRef.current.querySelectorAll("h1, h2, h3, h4, h5, h6");
    const target = Array.from(headings).find(
      (el) => (el.textContent ?? "").trim() === scrollToHeading,
    );
    if (target) {
      target.scrollIntoView({ behavior: "smooth", block: "start" });
      onScrolledToHeading?.();
    }
  }, [loading, renderedContent, scrollToHeading, onScrolledToHeading]);

  useLayoutEffect(() => {
    if (loading || !articleRef.current || !isMarkdown || isRawView || !searchQuery?.trim()) {
      setSearchHitMarkers([]);
      return;
    }

    const updateMarkers = () => {
      if (!articleRef.current) {
        return;
      }
      setSearchHitMarkers(collectSearchHitMarkers(articleRef.current, searchQuery));
    };

    updateMarkers();

    const resizeObserver = new ResizeObserver(() => updateMarkers());
    resizeObserver.observe(articleRef.current);
    for (const element of articleRef.current.querySelectorAll("img, svg")) {
      resizeObserver.observe(element);
    }

    return () => {
      resizeObserver.disconnect();
    };
  }, [loading, renderedContent, isMarkdown, isRawView, searchQuery]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-50 text-gh-text-secondary text-sm">
        Loading...
      </div>
    );
  }

  return (
    <div className="flex items-start gap-2">
      <article
        ref={articleRef}
        className={`markdown-body relative min-w-0 flex-1 overflow-visible${isWide ? " markdown-body--wide" : ""}`}
      >
        <div className="pointer-events-none absolute inset-0 z-10 overflow-visible">
          {searchHitMarkers.map((marker, index) => (
            <div
              key={`${marker.top}:${marker.height}:${index}`}
              className="absolute w-1 rounded-none bg-gh-text/80"
              style={{
                left: SEARCH_HIT_COLUMN_OFFSET,
                top: marker.top,
                height: marker.height,
              }}
            />
          ))}
        </div>
        {renderedContent}
      </article>
      <div className="shrink-0 flex flex-col gap-2 -mr-4 -mt-4">
        {isMarkdown && <TocToggle isTocOpen={isTocOpen} onToggle={onTocToggle} />}
        {isMarkdown && <RawToggle isRaw={isRawView} onToggle={() => setIsRawView((v) => !v)} />}
        <CopyButton content={content} />
        <CloseFileButton onClose={onRemoveFile} uploaded={uploaded} />
      </div>
    </div>
  );
}

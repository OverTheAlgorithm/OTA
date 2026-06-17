// Vercel Edge Middleware — server-side Open Graph / <title> injection.
//
// Why this exists:
// The site is a client-rendered SPA. Every URL is served the same static
// index.html, whose <title> and og: tags carry the default WizLetter branding.
// react-helmet-async only rewrites them AFTER JavaScript runs, which means:
//   1. Social crawlers (KakaoTalk, Slack, etc.) never run JS, so a shared
//      /topic or /editor-picks link always previewed the default WizLetter
//      image + title instead of the article's own image + headline.
//   2. The browser tab briefly (or, when Helmet misbehaves, permanently) showed
//      the default title / bare URL instead of the article headline.
//
// This middleware fetches the page's data at the edge and rewrites the head
// tags before the HTML is sent, fixing both the crawler preview and the tab
// title from the very first byte — no JS required.

export const config = {
  // Only article-style detail pages need per-resource metadata.
  matcher: ["/topic/:id", "/editor-picks/:id"],
};

const BACKEND = process.env.BACKEND_API_URL || "https://server.wizletter.com";
const SITE = "https://wizletter.com";
const DEFAULT_IMAGE = `${SITE}/w_logo.png`;
const BRAND_SUFFIX = "위즈레터";

interface PageMeta {
  title: string;
  description: string;
  image: string;
  url: string;
}

// ── helpers ──────────────────────────────────────────────────────────────────

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function stripHtml(html: string): string {
  return html
    .replace(/<[^>]*>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function truncate(text: string, max = 200): string {
  if (text.length <= max) return text;
  return `${text.slice(0, max - 1).trimEnd()}…`;
}

// Crawlers (KakaoTalk etc.) require an absolute https og:image. Topic images are
// stored as site-relative API paths; external editor-pick images are already
// absolute. Anything unusable falls back to the brand logo.
function absoluteImage(image: string | null | undefined): string {
  if (!image) return DEFAULT_IMAGE;
  if (/^https?:\/\//.test(image)) return image;
  if (image.startsWith("/")) return `${SITE}${image}`;
  return DEFAULT_IMAGE;
}

async function fetchTopicMeta(id: string, pageUrl: string): Promise<PageMeta | null> {
  const res = await fetch(`${BACKEND}/api/v1/context/topic/${id}`, {
    headers: { accept: "application/json" },
  });
  if (!res.ok) return null;
  const body = await res.json();
  const data = body?.data;
  if (!data?.topic) return null;
  return {
    title: `${data.topic} - ${BRAND_SUFFIX}`,
    description: truncate(data.detail || ""),
    image: absoluteImage(data.image_url),
    url: pageUrl,
  };
}

async function fetchEditorPickMeta(id: string, pageUrl: string): Promise<PageMeta | null> {
  const res = await fetch(`${BACKEND}/api/v1/editor-picks/${id}`, {
    headers: { accept: "application/json" },
  });
  if (!res.ok) return null;
  const body = await res.json();
  const data = body?.data;
  if (!data?.title) return null;
  return {
    title: `${data.title} | WizLetter`,
    description: truncate(stripHtml(data.content_html || "")),
    image: absoluteImage(data.first_image_url),
    url: pageUrl,
  };
}

// Replace the first tag matching `pattern` with `replacement`. If the tag is
// absent (defensive — the static template always carries it), append the
// replacement just before </head> so the meta still ships.
function replaceOrInjectHead(html: string, pattern: RegExp, replacement: string): string {
  if (pattern.test(html)) return html.replace(pattern, replacement);
  return html.replace("</head>", `    ${replacement}\n  </head>`);
}

function injectMeta(html: string, meta: PageMeta): string {
  const title = escapeHtml(meta.title);
  const description = escapeHtml(meta.description);
  const image = escapeHtml(meta.image);
  const url = escapeHtml(meta.url);

  let out = html;
  out = replaceOrInjectHead(out, /<title>[\s\S]*?<\/title>/, `<title>${title}</title>`);
  out = replaceOrInjectHead(
    out,
    /<meta\s+name="description"[\s\S]*?\/>/,
    `<meta name="description" content="${description}" />`,
  );
  out = replaceOrInjectHead(
    out,
    /<meta\s+property="og:title"[\s\S]*?\/>/,
    `<meta property="og:title" content="${title}" />`,
  );
  out = replaceOrInjectHead(
    out,
    /<meta\s+property="og:description"[\s\S]*?\/>/,
    `<meta property="og:description" content="${description}" />`,
  );
  out = replaceOrInjectHead(
    out,
    /<meta\s+property="og:image"[\s\S]*?\/>/,
    `<meta property="og:image" content="${image}" />`,
  );
  out = replaceOrInjectHead(
    out,
    /<meta\s+property="og:type"[\s\S]*?\/>/,
    `<meta property="og:type" content="article" />`,
  );
  // og:url is not in the static template — always inject it.
  out = out.replace("</head>", `    <meta property="og:url" content="${url}" />\n  </head>`);
  return out;
}

// ── handler ──────────────────────────────────────────────────────────────────

export default async function middleware(request: Request): Promise<Response> {
  const url = new URL(request.url);
  const segments = url.pathname.split("/").filter(Boolean);

  // Fetch the static SPA shell to rewrite. The matcher excludes /index.html,
  // so this never recurses back into the middleware.
  const shellRes = await fetch(new URL("/index.html", url.origin));
  const html = await shellRes.text();
  const passthrough = () =>
    new Response(html, { headers: { "content-type": "text/html; charset=utf-8" } });

  const id = segments[1];
  if (!id) return passthrough();

  const pageUrl = `${SITE}${url.pathname}`;
  let meta: PageMeta | null = null;
  try {
    if (segments[0] === "topic") {
      meta = await fetchTopicMeta(id, pageUrl);
    } else if (segments[0] === "editor-picks") {
      meta = await fetchEditorPickMeta(id, pageUrl);
    }
  } catch {
    // Backend hiccup — fall back to the default shell rather than erroring.
    meta = null;
  }

  if (!meta) return passthrough();

  return new Response(injectMeta(html, meta), {
    headers: {
      "content-type": "text/html; charset=utf-8",
      // Cache the rendered shell at the edge; new deploys bust it automatically.
      "cache-control": "public, s-maxage=300, stale-while-revalidate=600",
    },
  });
}

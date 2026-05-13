import DOMPurify from "dompurify";

// Defense in depth: the server already sanitises with bluemonday, but we
// re-sanitise on the client so a compromised cache, MITM, or older record
// cannot inject anything past our trust boundary.
//
// The allowlist mirrors the server's UGC policy.
const ALLOWED_TAGS = [
  "a", "p", "br", "span", "div",
  "h1", "h2", "h3", "h4", "h5", "h6",
  "strong", "em", "u", "s", "code", "pre", "blockquote",
  "ul", "ol", "li",
  "img",
];

const ALLOWED_ATTR = [
  "href", "target", "rel",
  "src", "alt", "title",
  "class", "style",
];

export function sanitizeContentHTML(html: string): string {
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    ALLOW_DATA_ATTR: false,
  });
}

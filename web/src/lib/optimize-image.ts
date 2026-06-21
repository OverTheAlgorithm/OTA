// Client-side image optimization run just before upload. Inline editor images
// (especially pasted screenshots) are often large, uncompressed PNGs; we
// downscale to a sane max dimension and re-encode to WebP in the browser so the
// server stores a much smaller file. The server still enforces size/type, so
// this is a best-effort optimization — any failure falls back to the original.

// Long-edge cap. Article body renders well under this width, so anything larger
// is wasted bytes.
const MAX_DIMENSION = 1600;
const WEBP_QUALITY = 0.82;

// Animated GIFs lose their animation when redrawn onto a canvas, so we leave
// them untouched.
const SKIP_TYPES = new Set<string>(["image/gif"]);

function canvasToBlob(
  canvas: HTMLCanvasElement,
  type: string,
  quality: number,
): Promise<Blob | null> {
  return new Promise((resolve) => canvas.toBlob(resolve, type, quality));
}

/**
 * Returns an optimized copy of the image (downscaled + WebP), or the original
 * file unchanged if optimization isn't possible/beneficial.
 */
export async function optimizeImage(file: File): Promise<File> {
  if (SKIP_TYPES.has(file.type)) return file;
  if (typeof createImageBitmap !== "function") return file;

  let bitmap: ImageBitmap;
  try {
    bitmap = await createImageBitmap(file);
  } catch {
    // Undecodable here (corrupt, or a format the canvas can't read) — let the
    // server deal with the original.
    return file;
  }

  try {
    const { width, height } = bitmap;
    const scale = Math.min(1, MAX_DIMENSION / Math.max(width, height));
    const targetW = Math.max(1, Math.round(width * scale));
    const targetH = Math.max(1, Math.round(height * scale));

    const canvas = document.createElement("canvas");
    canvas.width = targetW;
    canvas.height = targetH;
    const ctx = canvas.getContext("2d");
    if (!ctx) return file;
    ctx.drawImage(bitmap, 0, 0, targetW, targetH);

    const blob = await canvasToBlob(canvas, "image/webp", WEBP_QUALITY);
    // toBlob yields null when the browser can't encode WebP — keep the original.
    if (!blob) return file;

    // If we didn't downscale and WebP came out no smaller (e.g. an already
    // well-compressed image), the conversion isn't worth it.
    if (scale === 1 && blob.size >= file.size) return file;

    const baseName = file.name.replace(/\.[^.]+$/, "") || "image";
    return new File([blob], `${baseName}.webp`, { type: "image/webp" });
  } finally {
    bitmap.close?.();
  }
}

/**
 * Detect whether an ad blocker is active by attempting to load a bait script.
 * Returns a promise that resolves to `true` if ads are blocked.
 */
export function detectAdBlock(): Promise<boolean> {
  return new Promise((resolve) => {
    // Check 1: bait script flag
    if ((window as Record<string, unknown>).__ad_loaded__) {
      resolve(false);
      return;
    }

    // Check 2: try loading the bait script dynamically
    const script = document.createElement("script");
    script.src = "/ads.js?" + Date.now(); // cache-bust
    script.async = true;

    script.onload = () => {
      resolve(!(window as Record<string, unknown>).__ad_loaded__);
      script.remove();
    };

    script.onerror = () => {
      resolve(true);
      script.remove();
    };

    document.head.appendChild(script);
  });
}

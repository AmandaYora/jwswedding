import { useEffect } from "react";

// Ref-counted, because two overlays can legitimately be open at once
// (EvidenceListModal renders <Modal> and <EvidenceViewerModal> as siblings).
// A naive "set overflow hidden on mount, restore on unmount" would let the
// inner one's cleanup unlock the page while the outer is still covering it.
let lockCount = 0;
let restoreOverflow = "";
let restorePaddingRight = "";

function lock() {
  if (lockCount === 0) {
    const { body } = document;
    restoreOverflow = body.style.overflow;
    restorePaddingRight = body.style.paddingRight;
    // Removing the scrollbar reflows everything a few pixels wider; pad by
    // exactly the width it occupied so the page underneath doesn't jump.
    const scrollbarWidth = window.innerWidth - document.documentElement.clientWidth;
    if (scrollbarWidth > 0) {
      const current = parseFloat(window.getComputedStyle(body).paddingRight) || 0;
      body.style.paddingRight = `${current + scrollbarWidth}px`;
    }
    body.style.overflow = "hidden";
  }
  lockCount += 1;
}

function unlock() {
  lockCount = Math.max(0, lockCount - 1);
  if (lockCount === 0) {
    document.body.style.overflow = restoreOverflow;
    document.body.style.paddingRight = restorePaddingRight;
  }
}

/** Freezes page scroll behind an overlay for as long as `active` is true. */
export function useBodyScrollLock(active: boolean) {
  useEffect(() => {
    if (!active) return;
    lock();
    return unlock;
  }, [active]);
}

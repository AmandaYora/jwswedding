/**
 * Opens a PDF the backend returns as a blob, without tripping the popup blocker.
 *
 * The obvious spelling — `const blob = await fetch(); window.open(URL.createObjectURL(blob))`
 * — is what every PDF button in this app used, and it fails SILENTLY. A browser
 * only honours `window.open` while the click's transient user activation is
 * still alive; once an awaited request has come back, that window may already
 * have closed. The call then returns `null` instead of throwing, so a caller
 * that ignores the return value shows no tab, no error, nothing at all.
 *
 * That is exactly how the PO Paket "Pratinjau PDF" button failed: the request
 * itself answered 200 with a valid PDF, but the PO endpoint does noticeably
 * more work than the leaner Invoice one (the PO view, project, profile,
 * completeness check, logo, signature, payment ledger, and a cross-module
 * phone lookup, then a multi-page render), so it fell outside the activation
 * window that the Invoice download still slipped under.
 *
 * The fix is to claim the tab FIRST, while the gesture is still valid, then
 * point it at the blob once it arrives. Three fallbacks follow, so this can
 * never again fail without telling anyone:
 *
 *  1. Tab reserved successfully -> navigate it to the blob.
 *  2. Popup blocked anyway -> save the file instead. A download needs no popup
 *     permission, so the user still gets the document.
 *  3. Neither worked -> throw, so the caller surfaces a message.
 */
export async function openPdfInNewTab(
  fetchBlob: () => Promise<Blob>,
  fallbackFileName: string,
): Promise<void> {
  // Synchronous, before any await: this is the whole point.
  const reserved = window.open("", "_blank");

  let blob: Blob;
  try {
    blob = await fetchBlob();
  } catch (err) {
    // Don't strand an empty about:blank tab when the request fails.
    reserved?.close();
    throw err;
  }

  const url = URL.createObjectURL(blob);
  // Keep the URL alive long enough for the tab to load it, then release it.
  // Revoking immediately would blank the tab; never revoking leaks the whole
  // PDF for as long as the session lasts.
  const release = () => URL.revokeObjectURL(url);

  if (reserved && !reserved.closed) {
    reserved.location.href = url;
    window.setTimeout(release, 60_000);
    return;
  }

  const link = document.createElement("a");
  link.href = url;
  link.download = fallbackFileName.endsWith(".pdf") ? fallbackFileName : `${fallbackFileName}.pdf`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(release, 60_000);
}

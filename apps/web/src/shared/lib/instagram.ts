// instagramUrlFrom turns the free-text Sosial Media field into a real
// Instagram link, or null when it isn't one (PLAN revisi-vendor-venue-portal
// §1.1 poin 2–3 / §4.6). The field already holds values like "-", phone
// numbers, or multi-line notes — returning null keeps the IG button hidden
// instead of rendering a wrong link.
export function instagramUrlFrom(raw: string | null | undefined): string | null {
  if (!raw) return null;
  const trimmed = raw.trim();
  if (!trimmed) return null;

  // Full URL (or bare domain) already pointing at Instagram — force https.
  const urlMatch = trimmed.match(/^(https?:\/\/)?(www\.)?instagram\.com\/(.+)$/i);
  if (urlMatch) {
    const rest = urlMatch[3].trim();
    if (!rest) return null;
    return `https://instagram.com/${rest}`;
  }

  // @handle or plain handle (Instagram usernames: letters, digits, dots,
  // underscores, max 30 chars). Anything else (phone numbers with spaces/
  // dashes/plus, "-", notes) is not a handle — return null.
  //
  // One exception inside the handle shape: a dotted string ending in a common
  // domain suffix ("grandballroom.com") is a website, not a handle — and this
  // field really holds websites (the form placeholder itself suggests one).
  // Linking those to instagram.com/<domain> would be exactly the wrong link
  // rule 5 exists to prevent, so they return null. Handles with dots that
  // are NOT domain-like ("jws.wedding_official") still link.
  const handle = trimmed.startsWith("@") ? trimmed.slice(1).trim() : trimmed;
  if (/^[A-Za-z0-9._]{1,30}$/.test(handle)) {
    if (!trimmed.startsWith("@") && /\.(com|net|org|info|biz|id|co|io|me|xyz|site|online|store|my|sg)$/i.test(handle)) {
      return null;
    }
    // A long run of digits and nothing else is a phone number, not a handle —
    // this field really does hold bare numbers like "081312000326", and
    // linking those to instagram.com/081312000326 is exactly the wrong link
    // this function exists to avoid. Formats with spaces, "-" or "+" already
    // fail the handle shape above; this catches the bare-digits one.
    // Threshold 8 keeps genuinely short numeric handles ("2020") working,
    // while every Indonesian phone number (10–13 digits) is rejected.
    if (!trimmed.startsWith("@") && /^\d{8,}$/.test(handle)) {
      return null;
    }
    return `https://instagram.com/${handle}`;
  }
  return null;
}

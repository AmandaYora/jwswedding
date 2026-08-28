// Prefill helpers for Tambah Client's Username/Password fields — PLAN.md
// mom-25082026-item-sebagian item 10. Both are pure functions: the caller
// decides when to apply the result and the staff member can always edit
// the field afterward (D3/D4/D5 in the plan) — this module never talks to
// the store or the API.

// suggestUsername turns a client's full name into a "jws_"-prefixed
// username candidate that satisfies usernameSchema
// (@/shared/lib/validators.ts: ^[a-z0-9_.]+$, 4-32 chars). Non [a-z0-9]
// characters (spaces, punctuation, accents already normalized away by the
// caller's input) become "_", runs of "_" collapse to one, and a trailing
// "_" is trimmed so "Budi " doesn't end up as "jws_budi_". An empty name
// still yields "jws_" (4 chars), which clears the schema's minimum.
export function suggestUsername(fullName: string): string {
  const slug = fullName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return `jws_${slug}`.slice(0, 32);
}

// suggestPassword turns an ISO event date ("YYYY-MM-DD") into "DDMMYYYY" —
// 8 digits, clears clientCreateSchema's 6-character minimum. Returns "" for
// anything that isn't a well-formed YYYY-MM-DD string, so the field is left
// empty for manual entry instead of showing a nonsense password.
export function suggestPassword(eventDateISO: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(eventDateISO);
  if (!match) return "";
  const [, year, month, day] = match;
  return `${day}${month}${year}`;
}

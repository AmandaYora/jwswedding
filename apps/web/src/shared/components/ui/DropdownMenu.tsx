import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { cn } from "@/shared/lib/cn";

export interface DropdownMenuItem {
  label: string;
  onSelect: () => void;
  icon?: ReactNode;
  tone?: "default" | "danger";
  disabled?: boolean;
}

interface DropdownMenuProps {
  /** Tombol pemicu. Dibungkus span, jadi boleh komponen apa pun. */
  trigger: ReactNode;
  items: DropdownMenuItem[];
  align?: "start" | "end";
  /** Label aksesibilitas untuk pemicu. */
  label?: string;
  className?: string;
}

/**
 * Menu aksi generik untuk kolom Aksi pada tabel daftar.
 *
 * Domain-agnostic sesuai .claude/rules/frontend-react.md -- tidak tahu apa pun
 * soal rundown, project, atau vendor. Panelnya di-portal ke body dengan idiom
 * yang sama seperti Combobox (click-outside `mousedown` + createPortal), supaya
 * tidak terpotong oleh `overflow` milik pembungkus tabel.
 */
export function DropdownMenu({
  trigger,
  items,
  align = "end",
  label = "Menu aksi",
  className,
}: DropdownMenuProps) {
  const [open, setOpen] = useState(false);
  const [rect, setRect] = useState<{ top: number; left: number; width: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([]);

  const close = useCallback((returnFocus: boolean) => {
    setOpen(false);
    if (returnFocus) triggerRef.current?.focus({ preventScroll: true });
  }, []);

  // Navigasi panah. `role="menu"` menjanjikan perilaku ini kepada pembaca
  // layar dan pengguna keyboard; menyatakan peran itu tanpa menyediakannya
  // justru lebih menyesatkan daripada tidak menyatakannya sama sekali.
  const moveFocus = useCallback((to: 1 | -1 | "first" | "last") => {
    const items = itemRefs.current.filter((n): n is HTMLButtonElement => n !== null && !n.disabled);
    if (items.length === 0) return;
    const current = items.indexOf(document.activeElement as HTMLButtonElement);
    let next: number;
    if (to === "first") next = 0;
    else if (to === "last") next = items.length - 1;
    else if (current < 0) next = to === 1 ? 0 : items.length - 1;
    else next = (current + to + items.length) % items.length;
    // preventScroll: panel ini menutup diri saat halaman bergulir, dan focus()
    // yang menggulung halaman akan menutupnya seketika setelah dibuka.
    items[next].focus({ preventScroll: true });
  }, []);

  const place = useCallback(() => {
    const el = triggerRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const width = 224;
    setRect({
      top: r.bottom + 6,
      left: align === "end" ? r.right - width : r.left,
      width,
    });
  }, [align]);

  useLayoutEffect(() => {
    if (open) place();
  }, [open, place]);

  // Fokus pindah ke item pertama begitu menu terbuka, supaya pengguna keyboard
  // tidak perlu menebak di mana fokusnya berada.
  useEffect(() => {
    if (open && rect) moveFocus("first");
  }, [open, rect, moveFocus]);

  useEffect(() => {
    if (!open) return;
    const onMouseDown = (e: MouseEvent) => {
      const t = e.target as Node;
      if (triggerRef.current?.contains(t) || panelRef.current?.contains(t)) return;
      setOpen(false);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case "Escape":
          e.preventDefault();
          close(true);
          break;
        case "ArrowDown":
          e.preventDefault();
          moveFocus(1);
          break;
        case "ArrowUp":
          e.preventDefault();
          moveFocus(-1);
          break;
        case "Home":
          e.preventDefault();
          moveFocus("first");
          break;
        case "End":
          e.preventDefault();
          moveFocus("last");
          break;
      }
    };
    // Menu di dalam tabel bisa ikut tergeser saat halaman di-scroll; menutupnya
    // lebih jujur daripada membiarkan panel melayang lepas dari pemicunya.
    const onScrollOrResize = () => setOpen(false);
    document.addEventListener("mousedown", onMouseDown);
    document.addEventListener("keydown", onKeyDown);
    window.addEventListener("scroll", onScrollOrResize, true);
    window.addEventListener("resize", onScrollOrResize);
    return () => {
      document.removeEventListener("mousedown", onMouseDown);
      document.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("scroll", onScrollOrResize, true);
      window.removeEventListener("resize", onScrollOrResize);
    };
  }, [open, close, moveFocus]);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={label}
        onClick={() => setOpen((v) => !v)}
        className={cn("inline-flex items-center", className)}
      >
        {trigger}
      </button>
      {open &&
        rect &&
        createPortal(
          <div
            ref={panelRef}
            role="menu"
            style={{ position: "fixed", top: rect.top, left: rect.left, width: rect.width }}
            className="z-50 overflow-hidden rounded-md border border-border bg-white py-1 shadow-lg"
          >
            {items.map((item, i) => (
              <button
                key={item.label}
                ref={(el) => {
                  itemRefs.current[i] = el;
                }}
                type="button"
                role="menuitem"
                disabled={item.disabled}
                onClick={() => {
                  close(true);
                  item.onSelect();
                }}
                className={cn(
                  "flex w-full items-center gap-2 px-3 py-2 text-left text-[13px] transition-colors",
                  item.disabled
                    ? "cursor-not-allowed text-text-secondary/50"
                    : item.tone === "danger"
                      ? "text-danger hover:bg-danger/10"
                      : "text-text-primary hover:bg-surface-muted"
                )}
              >
                {item.icon}
                {item.label}
              </button>
            ))}
          </div>,
          document.body
        )}
    </>
  );
}

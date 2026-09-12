import {
  Children,
  isValidElement,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactElement,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown, Search } from "lucide-react";
import { cn } from "@/shared/lib/cn";

interface ComboboxOption {
  value: string;
  label: string;
  disabled?: boolean;
}

interface OptionElementProps {
  value?: string;
  children?: ReactNode;
  disabled?: boolean;
}

function getNodeText(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(getNodeText).join("");
  if (isValidElement(node)) return getNodeText((node.props as OptionElementProps).children);
  return "";
}

function optionsFromChildren(children: ReactNode): ComboboxOption[] {
  return Children.toArray(children)
    .filter((child): child is ReactElement<OptionElementProps> => isValidElement(child))
    .map((child) => ({
      value: String(child.props.value ?? ""),
      label: getNodeText(child.props.children),
      disabled: child.props.disabled,
    }));
}

// Panel geometry. The panel is portalled to <body> with `position: fixed`, so
// whatever falls past the bottom edge can never be scrolled back into view by
// the page: its height has to be clamped to the room the viewport actually
// offers, and it flips above the trigger when the room below is too tight.
const PANEL_GAP = 4;
const VIEWPORT_MARGIN = 8;
const SEARCH_ROW_HEIGHT = 47; // h-8 input + p-1.5 wrapper + bottom border
const OPTION_HEIGHT = 37;
const LIST_PADDING = 8;
const MAX_PANEL_HEIGHT = 320;
const MIN_USABLE_HEIGHT = 200; // below this, dropping up beats a cramped list
const MIN_PANEL_HEIGHT = 96; // last resort on very short viewports

interface PanelRect {
  left: number;
  width: number;
  maxHeight: number;
  /** Set when the panel drops down; `bottom` is set instead when it drops up. */
  top?: number;
  bottom?: number;
}

function measurePanel(trigger: HTMLElement, optionCount: number): PanelRect {
  const r = trigger.getBoundingClientRect();
  const below = window.innerHeight - r.bottom - PANEL_GAP - VIEWPORT_MARGIN;
  const above = r.top - PANEL_GAP - VIEWPORT_MARGIN;
  const desired = Math.min(
    MAX_PANEL_HEIGHT,
    SEARCH_ROW_HEIGHT + Math.max(optionCount, 1) * OPTION_HEIGHT + LIST_PADDING
  );
  const dropUp = below < desired && below < MIN_USABLE_HEIGHT && above > below;
  const room = dropUp ? above : below;
  const maxHeight = Math.max(Math.min(desired, room), MIN_PANEL_HEIGHT);
  const base = { left: r.left, width: r.width, maxHeight };
  // Anchoring a drop-up by `bottom` lets the panel shrink towards the trigger
  // as the query filters the list, instead of leaving a gap below it.
  return dropUp
    ? { ...base, bottom: window.innerHeight - r.top + PANEL_GAP }
    : { ...base, top: r.bottom + PANEL_GAP };
}

interface ComboboxProps {
  value: string;
  onChange: (e: { target: { value: string } }) => void;
  children: ReactNode;
  className?: string;
  disabled?: boolean;
  placeholder?: string;
}

export function Combobox({ value, onChange, children, className, disabled, placeholder = "Pilih..." }: ComboboxProps) {
  const options = useMemo(() => optionsFromChildren(children), [children]);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [highlighted, setHighlighted] = useState(0);
  const [rect, setRect] = useState<PanelRect | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  const selected = options.find((o) => o.value === value);
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (q === "") return options;
    return options.filter((o) => o.label.toLowerCase().includes(q));
  }, [options, query]);

  function openDropdown() {
    if (disabled) return;
    const trigger = triggerRef.current;
    if (trigger) setRect(measurePanel(trigger, options.length));
    setQuery("");
    setHighlighted(Math.max(0, options.findIndex((o) => o.value === value)));
    setOpen(true);
  }

  function closeDropdown() {
    setOpen(false);
  }

  function selectOption(opt: ComboboxOption) {
    if (opt.disabled) return;
    onChange({ target: { value: opt.value } });
    closeDropdown();
    triggerRef.current?.focus();
  }

  useEffect(() => {
    if (!open) return;
    requestAnimationFrame(() => searchRef.current?.focus());

    function onMouseDown(e: MouseEvent) {
      const target = e.target as Node;
      if (triggerRef.current?.contains(target) || panelRef.current?.contains(target)) return;
      closeDropdown();
    }
    function onViewportChange(e: Event) {
      // Scrolling the dropdown's own options list also fires a (capture-phase)
      // window scroll event — only a genuine outer/page scroll should close
      // the panel, since `rect` (its fixed position) is a one-time snapshot
      // from open time and would otherwise drift from the trigger.
      if (panelRef.current?.contains(e.target as Node)) return;
      closeDropdown();
    }

    document.addEventListener("mousedown", onMouseDown);
    window.addEventListener("scroll", onViewportChange, true);
    window.addEventListener("resize", onViewportChange);
    return () => {
      document.removeEventListener("mousedown", onMouseDown);
      window.removeEventListener("scroll", onViewportChange, true);
      window.removeEventListener("resize", onViewportChange);
    };
  }, [open]);

  // Now that the list scrolls, keyboard navigation has to drag it along —
  // otherwise arrowing past the visible rows highlights something off-screen.
  useEffect(() => {
    if (!open) return;
    const item = listRef.current?.querySelector<HTMLElement>(`[data-index="${highlighted}"]`);
    item?.scrollIntoView({ block: "nearest" });
  }, [open, highlighted, filtered.length]);

  function handleTriggerKeyDown(e: KeyboardEvent<HTMLButtonElement>) {
    if (!open && (e.key === "Enter" || e.key === " " || e.key === "ArrowDown" || e.key === "ArrowUp")) {
      e.preventDefault();
      openDropdown();
    }
  }

  function handleSearchKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlighted((h) => Math.min(filtered.length - 1, h + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlighted((h) => Math.max(0, h - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const opt = filtered[highlighted];
      if (opt) selectOption(opt);
    } else if (e.key === "Escape") {
      e.preventDefault();
      closeDropdown();
      triggerRef.current?.focus();
    }
  }

  return (
    <div className={cn("relative", className)}>
      <button
        ref={triggerRef}
        type="button"
        disabled={disabled}
        onClick={() => (open ? closeDropdown() : openDropdown())}
        onKeyDown={handleTriggerKeyDown}
        className={cn(
          "flex h-9 w-full items-center justify-between gap-2 rounded-md border border-border bg-white px-3 text-left text-sm text-text-primary transition-colors",
          "focus:outline-none focus:ring-2 focus:ring-navy-900/20 focus:border-navy-900",
          "disabled:cursor-not-allowed disabled:bg-surface-muted disabled:text-text-secondary",
          open && "border-navy-900 ring-2 ring-navy-900/20"
        )}
      >
        <span className={cn("truncate", !selected && "text-text-secondary/70")}>{selected ? selected.label : placeholder}</span>
        <ChevronDown className={cn("h-4 w-4 shrink-0 text-text-secondary transition-transform", open && "rotate-180")} />
      </button>

      {open &&
        rect &&
        createPortal(
          <div
            ref={panelRef}
            style={{
              position: "fixed",
              top: rect.top,
              bottom: rect.bottom,
              left: rect.left,
              width: rect.width,
              maxHeight: rect.maxHeight,
            }}
            className="z-50 flex flex-col overflow-hidden rounded-md border border-border bg-white shadow-lg"
          >
            <div className="relative shrink-0 border-b border-border-light p-1.5">
              <Search className="pointer-events-none absolute left-4 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-secondary" />
              <input
                ref={searchRef}
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setHighlighted(0);
                }}
                onKeyDown={handleSearchKeyDown}
                placeholder="Cari..."
                className="h-8 w-full rounded-md border-0 bg-surface-muted pl-8 pr-2 text-[13px] text-text-primary placeholder:text-text-secondary/70 focus:outline-none"
              />
            </div>
            <ul ref={listRef} role="listbox" className="min-h-0 flex-1 overflow-y-auto overscroll-contain py-1">
              {filtered.length === 0 ? (
                <li className="px-3 py-2.5 text-center text-[13px] text-text-secondary">Tidak ditemukan</li>
              ) : (
                filtered.map((opt, idx) => (
                  <li
                    key={opt.value}
                    role="option"
                    data-index={idx}
                    aria-selected={opt.value === value}
                    onMouseEnter={() => setHighlighted(idx)}
                    onClick={() => selectOption(opt)}
                    className={cn(
                      "flex cursor-pointer items-center justify-between gap-2 px-3 py-2 text-[13px]",
                      idx === highlighted && "bg-surface-muted",
                      opt.disabled && "pointer-events-none text-text-secondary/50"
                    )}
                  >
                    <span className={cn("truncate", opt.value === value && "font-semibold text-navy-900")}>{opt.label}</span>
                    {opt.value === value && <Check className="h-3.5 w-3.5 shrink-0 text-navy-900" />}
                  </li>
                ))
              )}
            </ul>
          </div>,
          document.body
        )}
    </div>
  );
}

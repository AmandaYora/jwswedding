import { NavLink } from "react-router-dom";
import { cn } from "@/shared/lib/cn";

interface TabItem {
  to: string;
  label: string;
  end?: boolean;
}

interface TabNavProps {
  items: TabItem[];
  className?: string;
  sticky?: boolean;
  /**
   * Drops the nav's own surface (tinted background + blur) and its bottom
   * border, for a host that already paints both — the Client Portal header
   * is one white/blurred bar whose own `border-b` sat directly under this
   * nav's, drawing a visible double rule, and whose white surface the
   * nav's `bg-background` tint broke with a grey band.
   *
   * `cn` here is a plain join, not tailwind-merge, so a caller cannot
   * cancel those classes from the outside via `className` — hence a prop.
   */
  bare?: boolean;
}

export function TabNav({ items, className, sticky = true, bare = false }: TabNavProps) {
  return (
    <nav
      className={cn(
        // Tabs overflow on narrow screens by design. Hiding the scrollbar
        // keeps the strip from gaining a chunk of height (and a grey bar)
        // on Windows/Chrome, where overlay scrollbars are not the default.
        "flex gap-6 overflow-x-auto px-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
        !bare && "border-b border-border bg-background/95 backdrop-blur",
        sticky && "sticky top-16 z-10",
        className
      )}
    >
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          className={({ isActive }) =>
            cn(
              "shrink-0 border-b-2 py-3 text-[13.5px] font-medium transition-colors",
              isActive ? "border-navy-900 font-semibold text-navy-900" : "border-transparent text-text-secondary hover:text-text-primary"
            )
          }
        >
          {item.label}
        </NavLink>
      ))}
    </nav>
  );
}

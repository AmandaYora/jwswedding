import { describe, it, expect, afterEach, vi } from "vitest";
import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { DropdownMenu } from "./DropdownMenu";

// globals mati di vitest.config.ts, jadi auto-cleanup milik RTL tidak
// terpasang sendiri — dibersihkan manual di sini.
afterEach(cleanup);

function renderMenu(onSelect = vi.fn()) {
  render(
    <DropdownMenu
      label="Aksi rundown"
      trigger={<span>Aksi</span>}
      items={[
        { label: "Generate PDF", onSelect },
        { label: "Generate DOCX", onSelect },
        { label: "Hapus", onSelect, tone: "danger" },
      ]}
    />
  );
  return { trigger: screen.getByRole("button", { name: "Aksi rundown" }), onSelect };
}

describe("DropdownMenu", () => {
  it("tertutup sampai pemicunya diklik", () => {
    renderMenu();
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("membuka panel dan memindahkan fokus ke item pertama", () => {
    const { trigger } = renderMenu();
    fireEvent.click(trigger);
    expect(screen.getByRole("menu")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Generate PDF" }));
  });

  it("menutup saat klik di luar panel", () => {
    const { trigger } = renderMenu();
    fireEvent.click(trigger);
    expect(screen.getByRole("menu")).toBeTruthy();

    // mousedown, bukan click: itu peristiwa yang benar-benar didengarkan
    // komponen (idiom yang sama dengan Combobox).
    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("tidak menutup saat klik di dalam panel", () => {
    const { trigger } = renderMenu();
    fireEvent.click(trigger);
    fireEvent.mouseDown(screen.getByRole("menuitem", { name: "Generate DOCX" }));
    expect(screen.getByRole("menu")).toBeTruthy();
  });

  it("menutup saat Escape dan mengembalikan fokus ke pemicu", () => {
    const { trigger } = renderMenu();
    fireEvent.click(trigger);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("panah bawah dan atas berputar di antara item", () => {
    const { trigger } = renderMenu();
    fireEvent.click(trigger);
    const [pdf, docx, hapus] = screen.getAllByRole("menuitem");

    fireEvent.keyDown(document, { key: "ArrowDown" });
    expect(document.activeElement).toBe(docx);
    fireEvent.keyDown(document, { key: "End" });
    expect(document.activeElement).toBe(hapus);
    // Dari item terakhir, panah bawah kembali ke item pertama.
    fireEvent.keyDown(document, { key: "ArrowDown" });
    expect(document.activeElement).toBe(pdf);
    fireEvent.keyDown(document, { key: "ArrowUp" });
    expect(document.activeElement).toBe(hapus);
  });

  it("melewati item yang dinonaktifkan", () => {
    const onSelect = vi.fn();
    render(
      <DropdownMenu
        label="Aksi"
        trigger={<span>Aksi</span>}
        items={[
          { label: "Menyiapkan...", onSelect, disabled: true },
          { label: "Hapus", onSelect },
        ]}
      />
    );
    fireEvent.click(screen.getByRole("button", { name: "Aksi" }));
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Hapus" }));
  });

  it("menjalankan aksi lalu menutup panel", () => {
    const { trigger, onSelect } = renderMenu();
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("menuitem", { name: "Hapus" }));
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menu")).toBeNull();
  });
});

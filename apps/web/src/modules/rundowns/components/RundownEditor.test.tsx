import { describe, it, expect, afterEach, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createMemoryRouter, RouterProvider, useParams } from "react-router-dom";
import { RundownEditor } from "@/modules/rundowns/components/RundownEditor";
import { RowEditor } from "@/modules/rundowns/components/RowEditor";
import { templateToDetail } from "@/modules/rundowns/pages/RundownTemplatePage";
import { RUNDOWN_TABS } from "@/modules/rundowns/schemas/rundown.schema";
import type { RundownDetail } from "@/modules/rundowns/types";
import type { RundownTab } from "@/app/routes/route-paths";

// globals mati di vitest.config.ts — auto-cleanup RTL dipasang manual.
afterEach(cleanup);

// Router data (createMemoryRouter) membuat `new Request(url, { signal })`
// setiap navigasi. Di jsdom, AbortSignal milik jsdom ditolak oleh Request
// bawaan Node (undici): "Expected signal to be an instance of AbortSignal".
// Itu murni ketidakcocokan lingkungan tes, bukan perilaku browser — sinyalnya
// dibuang di sini supaya navigasi bisa diuji.
const NativeRequest = globalThis.Request;
globalThis.Request = class extends NativeRequest {
  constructor(input: RequestInfo | URL, init?: RequestInit) {
    const { signal: _signal, ...rest } = init ?? {};
    super(input, rest);
  }
} as typeof Request;

function fixture(): RundownDetail {
  const d = templateToDetail({
    roles: [{ roleLabel: "Saksi", personName: "", note: "" }],
    committees: [],
    makeupRooms: [],
    itemsAkad: [],
    itemsResepsi: [],
    layoutNotes: [],
    updatedAt: null,
  });
  return { ...d, id: "1", projectName: "Dinda & Reza" };
}

function Harness({
  tab,
  onSave,
  value,
}: {
  tab: RundownTab;
  onSave: (tab: RundownTab) => Promise<void>;
  value: RundownDetail;
}) {
  return (
    <RundownEditor
      value={value}
      tabs={RUNDOWN_TABS}
      tab={tab}
      pathFor={(t) => `/r/${t}`}
      onSaveSection={(t) => onSave(t)}
      header={() => <h1>Editor</h1>}
    />
  );
}

function renderEditor(onSave: (tab: RundownTab) => Promise<void>) {
  const value = fixture();
  const router = createMemoryRouter(
    [
      {
        path: "/r/:tab",
        Component: function Route() {
          const { tab = "roles" } = useParams<{ tab: RundownTab }>();
          return <Harness tab={tab} onSave={onSave} value={value} />;
        },
      },
    ],
    { initialEntries: ["/r/roles"] }
  );
  render(<RouterProvider router={router} />);
  return router;
}

function editFirstRole() {
  fireEvent.change(screen.getByLabelText("Peran baris 1"), { target: { value: "Saksi CPW" } });
}

describe("RundownEditor — simpan saat transisi", () => {
  it("pindah tab dengan perubahan: menyimpan sekali lalu berpindah", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    const router = renderEditor(onSave);
    editFirstRole();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /Panitia Keluarga/ }));
    });

    await waitFor(() => expect(router.state.location.pathname).toBe("/r/committees"));
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledWith("roles");
  });

  it("tanpa perubahan: berpindah tanpa menyimpan", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    const router = renderEditor(onSave);

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /Panitia Keluarga/ }));
    });

    await waitFor(() => expect(router.state.location.pathname).toBe("/r/committees"));
    expect(onSave).not.toHaveBeenCalled();
  });

  it("gagal menyimpan: tetap di seksi dan menawarkan pilihan", async () => {
    const onSave = vi.fn().mockRejectedValue(new Error("server mati"));
    const router = renderEditor(onSave);
    editFirstRole();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /Panitia Keluarga/ }));
    });

    await waitFor(() => expect(screen.getByText("Perubahan belum tersimpan")).toBeTruthy());
    expect(router.state.location.pathname).toBe("/r/roles");

    // "Tetap di sini" membatalkan navigasi; suntingan masih ada.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Tetap di sini" }));
    });
    expect(router.state.location.pathname).toBe("/r/roles");
    expect((screen.getByLabelText("Peran baris 1") as HTMLInputElement).value).toBe("Saksi CPW");
  });

  it("isian tidak valid tidak dikirim ke server", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    renderEditor(onSave);
    fireEvent.change(screen.getByLabelText("Peran baris 1"), { target: { value: "a".repeat(151) } });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Simpan" }));
    });

    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByText("Maksimal 150 karakter")).toBeTruthy();
  });
});

describe("RowEditor — saklar Tanpa nomor", () => {
  it("mencentang menulis penanda, melepas mengosongkannya", () => {
    const onChange = vi.fn();
    const rows = [{ noLabel: "" }];
    const { rerender } = render(
      <RowEditor
        columns={[{ key: "noLabel", label: "Tanpa nomor", kind: "checkbox", checkbox: { on: "-", off: "" } }]}
        rows={rows}
        onChange={onChange}
        blank={() => ({ noLabel: "" })}
      />
    );
    const box = screen.getByLabelText("Tanpa nomor baris 1") as HTMLInputElement;
    expect(box.checked).toBe(false);
    fireEvent.click(box);
    expect(onChange).toHaveBeenLastCalledWith([{ noLabel: "-" }]);

    rerender(
      <RowEditor
        columns={[{ key: "noLabel", label: "Tanpa nomor", kind: "checkbox", checkbox: { on: "-", off: "" } }]}
        rows={[{ noLabel: "-" }]}
        onChange={onChange}
        blank={() => ({ noLabel: "" })}
      />
    );
    const checked = screen.getByLabelText("Tanpa nomor baris 1") as HTMLInputElement;
    expect(checked.checked).toBe(true);
    fireEvent.click(checked);
    expect(onChange).toHaveBeenLastCalledWith([{ noLabel: "" }]);
  });
});

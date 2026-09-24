import { describe, it, expect } from "vitest";
import { previewNumbers, isNoNumber } from "@/modules/rundowns/lib/numbering";
import { fieldErrorsFromServer, validateSectionPayload } from "@/modules/rundowns/lib/section-errors";
import { generateMessage } from "@/modules/rundowns/lib/generate-message";
import { mergeVendors } from "@/modules/rundowns/components/ProjectResyncDialog";
import { templateSummary } from "@/modules/rundowns/components/CreateRundownDialog";
import { RUNDOWN_TABS, TEMPLATE_TABS } from "@/modules/rundowns/schemas/rundown.schema";

const row = (noLabel: string) => ({ noLabel });

describe("previewNumbers — cermin renumberItems di server", () => {
  it("menomori berurutan dan melewati baris tanpa nomor", () => {
    expect(previewNumbers([row("-"), row(""), row("9"), row("-"), row("")])).toEqual(["", "1", "2", "", "3"]);
  });

  it("mengabaikan nomor lama yang dikirim klien", () => {
    expect(previewNumbers([row("3"), row("3"), row("3")])).toEqual(["1", "2", "3"]);
  });

  it("penanda tanpa nomor boleh berspasi", () => {
    expect(isNoNumber(" - ")).toBe(true);
    expect(isNoNumber("")).toBe(false);
  });
});

describe("fieldErrorsFromServer", () => {
  it("mengubah kunci berindeks server menjadi path bertitik", () => {
    expect(
      fieldErrorsFromServer({
        "items[3].timeLabel": ["Maksimal 50 karakter"],
        "cover.brideName": ["Maksimal 255 karakter"],
        "makeupRooms[0].lines[2].style": ["Gaya baris tidak dikenal"],
      })
    ).toEqual({
      "items.3.timeLabel": "Maksimal 50 karakter",
      "cover.brideName": "Maksimal 255 karakter",
      "makeupRooms.0.lines.2.style": "Gaya baris tidak dikenal",
    });
  });

  it("aman untuk respons tanpa errors", () => {
    expect(fieldErrorsFromServer(undefined)).toEqual({});
    expect(fieldErrorsFromServer(null)).toEqual({});
  });
});

describe("validateSectionPayload", () => {
  const acara = (timeLabel: string) => ({ noLabel: "", timeLabel, item: "", pic: "", note: "" });

  it("menandai baris yang tepat dengan pesan berbahasa Indonesia", () => {
    const errors = validateSectionPayload("acara-akad", { items: [acara("08.00"), acara("a".repeat(51))] });
    expect(errors).toEqual({ "items.1.timeLabel": "Maksimal 50 karakter" });
  });

  it("payload yang sah tidak menghasilkan galat", () => {
    expect(validateSectionPayload("acara-akad", { items: [acara("08.00 - 09.00")] })).toEqual({});
  });

  it("pilihan di luar daftar ditolak", () => {
    const errors = validateSectionPayload("layout", { layoutNotes: [{ kind: "Lain", numberLabel: "", content: "" }] });
    expect(errors["layoutNotes.0.kind"]).toBe("Pilihan tidak dikenal");
  });
});

describe("generateMessage", () => {
  it("memberi tahu bahwa klien tetap melihat versi terbaru", () => {
    const msg = generateMessage("pdf", "shared");
    expect(msg.tone).toBe("success");
    expect(msg.text).toContain("Klien tetap melihat");
    expect(msg.documentsLink).toBe(true);
  });

  it("arsip gagal tampil sebagai peringatan tanpa tautan", () => {
    const msg = generateMessage("docx", "failed");
    expect(msg.tone).toBe("danger");
    expect(msg.documentsLink).toBe(false);
  });
});

describe("mergeVendors", () => {
  const v = (categoryLabel: string, vendorName: string) => ({ categoryLabel, vendorName });

  it("menyisipkan setelah baris terakhir berkategori sama, sisanya di akhir", () => {
    const merged = mergeVendors(
      [v("DEKORASI", "A"), v("RIAS", "B"), v("RIAS", "C"), v("MC", "D")],
      [v("RIAS", "E"), v("FOTO", "F")]
    );
    expect(merged.map((x) => x.vendorName)).toEqual(["A", "B", "C", "E", "D", "F"]);
  });
});

describe("templateSummary", () => {
  it("hanya menyebut seksi yang berisi", () => {
    expect(
      templateSummary({
        roles: [{ roleLabel: "Saksi", personName: "", note: "" }],
        committees: [],
        makeupRooms: [],
        itemsAkad: [
          { noLabel: "", timeLabel: "", item: "a", pic: "", note: "" },
          { noLabel: "", timeLabel: "", item: "b", pic: "", note: "" },
        ],
        itemsResepsi: [],
        layoutNotes: [],
        updatedAt: null,
      })
    ).toEqual(["1 peran", "2 acara akad"]);
  });
});

describe("TEMPLATE_TABS", () => {
  it("tepat enam seksi, semuanya bagian dari RUNDOWN_TABS", () => {
    const all = new Set(RUNDOWN_TABS.map((t) => t.key));
    expect(TEMPLATE_TABS.map((t) => t.key)).toEqual([
      "roles",
      "committees",
      "makeup",
      "acara-akad",
      "acara-resepsi",
      "layout",
    ]);
    expect(TEMPLATE_TABS.every((t) => all.has(t.key))).toBe(true);
  });
});

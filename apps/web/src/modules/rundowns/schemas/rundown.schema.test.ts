import { describe, it, expect } from "vitest";
import {
  createRundownSchema,
  coverSchema,
  vendorRowSchema,
  menuItemSchema,
  layoutNoteSchema,
  dataLainnyaSchema,
  acaraItemSchema,
  RUNDOWN_TABS,
} from "./rundown.schema";

describe("createRundownSchema", () => {
  it("menolak project yang kosong", () => {
    const res = createRundownSchema.safeParse({ projectId: "" });
    expect(res.success).toBe(false);
    if (!res.success) {
      expect(res.error.issues[0]?.message).toBe("Project wajib dipilih");
    }
  });

  it("mengisi default untuk field opsional", () => {
    const res = createRundownSchema.parse({ projectId: "42" });
    expect(res).toEqual({
      projectId: "42",
      woPicName: "",
      woPicPhone: "",
      eventTimeLabel: "",
    });
  });
});

describe("batas panjang", () => {
  // Batas yang sama dijaga server (checkLen di rundown_service.go). Yang di
  // sini hanya supaya pengguna tahu lebih cepat; kalau salah satunya berubah,
  // ubah keduanya.
  it.each([
    ["woPicName", 100],
    ["woPicPhone", 100],
  ] as const)("cover.%s maksimal %i karakter", (field, max) => {
    expect(coverSchema.safeParse({ [field]: "a".repeat(max) }).success).toBe(true);
    expect(coverSchema.safeParse({ [field]: "a".repeat(max + 1) }).success).toBe(false);
  });

  it("vendorRow.categoryLabel maksimal 100 karakter", () => {
    expect(vendorRowSchema.safeParse({ categoryLabel: "a".repeat(101) }).success).toBe(false);
  });

  it("tableClothNote maksimal 255 karakter", () => {
    expect(dataLainnyaSchema.safeParse({ tableClothNote: "a".repeat(255) }).success).toBe(true);
    expect(dataLainnyaSchema.safeParse({ tableClothNote: "a".repeat(256) }).success).toBe(false);
  });
});

describe("enum seksi", () => {
  it("menolak groupKey dan style yang tidak dikenal", () => {
    expect(menuItemSchema.safeParse({ groupKey: "Prasmanan", style: "Numbered" }).success).toBe(false);
    expect(menuItemSchema.safeParse({ groupKey: "Stall", style: "Miring" }).success).toBe(false);
    expect(menuItemSchema.safeParse({ groupKey: "Stall", style: "Numbered" }).success).toBe(true);
  });

  it("menolak jenis catatan layout yang tidak dikenal", () => {
    expect(layoutNoteSchema.safeParse({ kind: "Catatan" }).success).toBe(false);
    expect(layoutNoteSchema.safeParse({ kind: "Rule" }).success).toBe(true);
  });
});

describe("acaraItemSchema", () => {
  it("membiarkan noLabel apa adanya — server yang menomori ulang", () => {
    const res = acaraItemSchema.parse({ noLabel: "99" });
    expect(res.noLabel).toBe("99");
  });

  it("timeLabel maksimal 50 karakter", () => {
    expect(acaraItemSchema.safeParse({ timeLabel: "a".repeat(51) }).success).toBe(false);
  });
});

describe("RUNDOWN_TABS", () => {
  // Dua belas, bukan sepuluh seperti cara WO menghitung halaman: Cover berdiri
  // sendiri dan SUSUNAN ACARA dipecah akad/resepsi.
  it("berisi dua belas kunci seksi yang unik", () => {
    const keys = RUNDOWN_TABS.map((t) => t.key);
    expect(keys).toHaveLength(12);
    expect(new Set(keys).size).toBe(12);
  });
});

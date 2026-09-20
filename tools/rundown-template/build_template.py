#!/usr/bin/env python3
"""Membangun rundown_template.docx dari DRAFT RUNDOWN DINDA & REZA.docx.

PLAN.md T1 menuliskan langkah ini sebagai pekerjaan manual di Word. Dikerjakan
lewat skrip supaya reproducible: kalau template perlu dibangun ulang (mis.
berkas sumber direvisi), jalankan ulang skrip ini alih-alih mengulang 8 langkah
manual yang rawan salah ketik.

Yang TIDAK boleh dilakukan di sini (PLAN.md §3):
  - menulis ulang seluruh XML dengan serializer yang mengarang prefix namespace
    -> dipakai lxml, yang mempertahankan prefix asli apa adanya;
  - menaruh placeholder terpecah antar-run -> setiap placeholder ditulis ke w:t
    PERTAMA milik paragraf, sisa run dikosongkan.

Jalankan:  python tools/rundown-template/build_template.py
"""
import copy
import io
import os
import struct
import sys
import zipfile

from lxml import etree as ET

W = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
A = "{http://schemas.openxmlformats.org/drawingml/2006/main}"
WP = "{http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing}"

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "source.docx")
OUT = os.path.join(
    HERE, "..", "..", "apps", "api", "internal", "modules", "rundowns",
    "presentation", "templates", "rundown_template.docx",
)
LAYOUT_IMAGE_ENTRY = "word/media/image10.png"


# --------------------------------------------------------------------------
# Utilitas paragraf
# --------------------------------------------------------------------------
def ptext(p):
    return "".join(t.text or "" for t in p.iter(W + "t"))


def set_ptext(p, value):
    """Tulis `value` ke w:t pertama, kosongkan sisanya.

    Kalau paragraf sama sekali belum punya run (sel kosong -- PLAN.md T1 poin
    5), run baru dibuat dengan menyalin rPr dari pPr/rPr supaya formatting
    paragrafnya tetap terpakai.
    """
    ts = list(p.iter(W + "t"))
    if not ts:
        r = ET.SubElement(p, W + "r")
        pr = p.find(W + "pPr")
        if pr is not None and pr.find(W + "rPr") is not None:
            r.append(copy.deepcopy(pr.find(W + "rPr")))
        t = ET.SubElement(r, W + "t")
        ts = [t]
    ts[0].text = value
    ts[0].set("{http://www.w3.org/XML/1998/namespace}space", "preserve")
    for t in ts[1:]:
        t.text = ""
    return p


def has_sect_break(p):
    """Paragraf ini menutup sebuah section (w:pPr/w:sectPr)?

    Dokumen sumber memisahkan halaman dengan 13 SECTION BREAK, bukan dengan
    `w:br w:type="page"`. Salah satunya (setelah halaman VENDORS) adalah
    satu-satunya tempat `headerReference` kop monogram dideklarasikan; section
    sesudahnya mewarisinya. Menghapus paragraf semacam ini membuang pemisah
    halaman DAN kop logo sekaligus.
    """
    pr = p.find(W + "pPr")
    return pr is not None and pr.find(W + "sectPr") is not None


def drop_para(p):
    """Buang paragraf, tetapi pertahankan section break yang menempel padanya."""
    parent = p.getparent()
    if parent is None:
        return
    if has_sect_break(p):
        keeper = ET.Element(W + "p")
        pr = ET.SubElement(keeper, W + "pPr")
        pr.append(copy.deepcopy(p.find(W + "pPr").find(W + "sectPr")))
        parent.replace(p, keeper)
        return
    parent.remove(p)


def marker_para(text):
    """Paragraf yang isinya hanya penanda region -- tidak ikut tercetak."""
    p = ET.Element(W + "p")
    r = ET.SubElement(p, W + "r")
    t = ET.SubElement(r, W + "t")
    t.text = text
    t.set("{http://www.w3.org/XML/1998/namespace}space", "preserve")
    return p


# Kolam numId untuk penomoran yang mengulang dari 1 di tiap ruangan makeup.
#
# Di berkas asli setiap ruangan memakai numId berbeda (2, 8, 15, ...) -- itulah
# yang membuat daftarnya mulai dari 1 lagi. Begitu satu baris ruangan digandakan
# oleh renderer, seluruh salinan berbagi numId yang sama dan Word melanjutkan
# hitungannya. Karena numbering.xml harus tetap identik byte demi byte saat
# render, kolamnya disiapkan SEKARANG, bukan ditambahkan saat generate.
MAKEUP_ROOM_POOL = 16
NUMBERED_POOL_BASE = 900   # menyalin abstractNumId milik numId 2
DASH_POOL_BASE = 920       # menyalin abstractNumId milik numId 17


def num_pool_xml(numbering_xml):
    """Sisipkan kolam <w:num> ke numbering.xml."""
    import re as _re

    def abstract_of(num_id):
        m = _re.search(
            r'<w:num w:numId="%d"><w:abstractNumId w:val="(\d+)"/></w:num>' % num_id,
            numbering_xml)
        if not m:
            raise SystemExit("numId %d tidak ditemukan di numbering.xml" % num_id)
        return m.group(1)

    # startOverride WAJIB: dua <w:num> yang menunjuk abstractNum yang sama
    # tetap BERBAGI hitungan di Word, jadi tanpa ini ruangan kedua mulai dari
    # "2." meski numId-nya sudah berbeda. Sudah terbukti saat render percobaan.
    override = ('<w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/>'
                '</w:lvlOverride>')
    extra = []
    for base, src in ((NUMBERED_POOL_BASE, 2), (DASH_POOL_BASE, 17)):
        abstract = abstract_of(src)
        for i in range(MAKEUP_ROOM_POOL):
            extra.append(
                '<w:num w:numId="%d"><w:abstractNumId w:val="%s"/>%s</w:num>'
                % (base + i, abstract, override))
    return numbering_xml.replace("</w:numbering>", "".join(extra) + "</w:numbering>")


def strip_highlight(el):
    """Buang seluruh sorotan warna.

    35 sorotan kuning dan 2 hijau di berkas contoh adalah coretan kerja WO
    ("IJAB TIDAK DISANDING", item stall yang belum fix), bukan elemen desain.
    Kalau dibiarkan, warnanya ikut tercetak di rundown SETIAP pasangan.
    """
    n = 0
    for rpr in el.iter(W + "rPr"):
        for e in rpr.findall(W + "highlight"):
            rpr.remove(e)
            n += 1
    return n


def numbered_para(paras):
    """Paragraf contoh berpenomoran angka (numId 2)."""
    for p in paras:
        pr = p.find(W + "pPr")
        if pr is None:
            continue
        npr = pr.find(W + "numPr")
        if npr is not None and npr.find(W + "numId") is not None \
                and npr.find(W + "numId").get(W + "val") == "2":
            return p
    raise SystemExit("paragraf berpenomoran (numId 2) tidak ditemukan")


def dash_para(paras):
    """Paragraf contoh bertanda hubung + miring (numId 17)."""
    for p in paras:
        pr = p.find(W + "pPr")
        if pr is None:
            continue
        npr = pr.find(W + "numPr")
        if npr is not None and npr.find(W + "numId") is not None \
                and npr.find(W + "numId").get(W + "val") == "17":
            return p
    raise SystemExit("paragraf bertanda hubung (numId 17) tidak ditemukan")


def strip_strikethrough(el):
    n = 0
    for rpr in el.iter(W + "rPr"):
        for tag in (W + "strike", W + "dstrike"):
            for e in rpr.findall(tag):
                rpr.remove(e)
                n += 1
    return n


def normalize_borders(tbl, sz="12"):
    n = 0
    for borders in tbl.iter(W + "tblBorders"):
        for b in borders:
            if b.get(W + "val") not in (None, "nil", "none"):
                b.set(W + "sz", sz)
                b.set(W + "val", "single")
                b.set(W + "color", "000000")
                n += 1
    return n


def tag_row(tr, open_tag, close_tag, cell_tokens):
    """Jadikan satu w:tr sebagai baris-template yang berulang."""
    cells = tr.findall(W + "tc")
    assert len(cells) == len(cell_tokens), (len(cells), len(cell_tokens))
    for tc, token in zip(cells, cell_tokens):
        paras = tc.findall(W + "p")
        target = next((p for p in paras if ptext(p).strip()), paras[-1])
        set_ptext(target, token)
        for p in paras:
            if p is not target:
                set_ptext(p, "")
    first = cells[0].findall(W + "p")
    set_ptext(
        next((p for p in first if ptext(p).strip()), first[-1]),
        open_tag + ptext(next((p for p in first if ptext(p).strip()), first[-1])),
    )
    last = cells[-1].findall(W + "p")
    tgt = next((p for p in last if ptext(p).strip()), last[-1])
    set_ptext(tgt, ptext(tgt) + close_tag)
    return tr


def keep_header_and_one_row(tbl, header_rows, open_tag, close_tag, cell_tokens,
                            model_row_index=None):
    """Sisakan baris header + satu baris-template, buang sisanya."""
    rows = tbl.findall(W + "tr")
    model_idx = header_rows if model_row_index is None else model_row_index
    model = rows[model_idx]
    tag_row(model, open_tag, close_tag, cell_tokens)
    for r in rows:
        if r is not model and rows.index(r) >= header_rows:
            tbl.remove(r)
    return model


def styled_region(container, anchor, name, styles, remove):
    """Ganti sekumpulan paragraf dengan satu region paragraf ber-style.

    `styles` = [(nama_style, paragraf_contoh)] -- tiap paragraf contoh dipakai
    apa adanya sebagai cetakan, hanya teksnya diganti penanda `{{~nama}}`.
    `remove` = paragraf yang dibuang setelah region disisipkan.
    """
    idx = list(container).index(anchor)
    block = [marker_para("{{#%s}}" % name)]
    for style_name, model in styles:
        p = copy.deepcopy(model)
        set_ptext(p, "{{~%s}}" % style_name)
        block.append(p)
    block.append(marker_para("{{/%s}}" % name))
    for offset, p in enumerate(block):
        container.insert(idx + offset, p)
    for p in remove:
        drop_para(p)
    return block


def neutral_layout_png(width, height):
    """PNG abu-abu polos seukuran diagram asli (PLAN.md §6.5 poin 3).

    Gambar contoh Dinda & Reza TIDAK boleh tertinggal di template -- ia akan
    ikut tercetak di rundown pasangan lain yang belum meng-upload denahnya.
    """
    import zlib

    raw = bytearray()
    for y in range(height):
        raw.append(0)
        for x in range(width):
            edge = x < 2 or y < 2 or x >= width - 2 or y >= height - 2
            v = 160 if edge else 244
            raw += bytes((v, v, v))

    def chunk(tag, data):
        return (struct.pack(">I", len(data)) + tag + data
                + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF))

    return (b"\x89PNG\r\n\x1a\n"
            + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(bytes(raw), 9))
            + chunk(b"IEND", b""))


# --------------------------------------------------------------------------
def build():
    zin = zipfile.ZipFile(SRC)
    root = ET.fromstring(zin.read("word/document.xml"))
    body = root.find(W + "body")
    kids = list(body)
    log = []

    # ---------------------------------------------------------------- cover
    cover = {
        4: "{{groom_name}}", 5: "{{groom_birth_order}}", 6: "{{groom_parents}}",
        8: "{{bride_name}}", 9: "{{bride_birth_order}}", 10: "{{bride_parents}}",
        13: "{{event_date_label}}", 14: "{{venue_label}}", 15: "{{event_time_label}}",
    }
    for i, token in cover.items():
        set_ptext(kids[i], token)
    log.append("cover: %d placeholder" % len(cover))

    # -------------------------------------------------------------- vendors
    # Daftar vendor adalah rangkaian paragraf (bukan tabel), dan satu kategori
    # bisa punya beberapa vendor -- lihat "VENUE & CATERING" (2) dan
    # "RIAS & BUSANA" (3). Region ber-style menangani keduanya: renderer yang
    # memutuskan kapan baris kategori dicetak.
    styled_region(
        body, kids[29], "vendors",
        [("category", kids[29]), ("vendor", kids[30]), ("spacer", kids[32])],
        remove=kids[29:65],
    )
    # kids[65:69] SENGAJA di luar `remove`: itu blok ORGANIZED BY (baris
    # kosong, judul, "JWS WEDDING", lalu kontak PIC). Versi pertama skrip ini
    # membuangnya bersama daftar vendor -- akibatnya kolom wo_pic_name/
    # wo_pic_phone yang diisi WO tidak punya tempat tercetak sama sekali.
    # Judul dan nama perusahaan tetap teks statis (seluruh template memang
    # ber-merek JWS: kop monogram, sampul); yang menjadi placeholder hanya
    # baris kontaknya, sebagai SATU skalar supaya kurung dan garis miring
    # tidak ikut tercetak saat kedua kolomnya masih kosong.
    assert ptext(kids[66]).strip() == "ORGANIZED BY", ptext(kids[66])
    set_ptext(kids[68], "{{wo_contact}}")
    log.append("vendors: region ber-style + blok ORGANIZED BY dipertahankan")

    # ------------------------------------------------------------ list nama
    t1 = kids[75]
    log.append("list nama: strikethrough dibuang=%d" % strip_strikethrough(t1))
    keep_header_and_one_row(
        t1, 1, "{{#roles}}", "{{/roles}}",
        ["{{role_label}}", "{{person_name}}", "{{note}}"],
    )

    # ------------------------------------------------------ panitia keluarga
    keep_header_and_one_row(
        kids[80], 1, "{{#committees}}", "{{/committees}}",
        ["{{role_label}}", "{{person_text}}", "{{job_desc}}"],
    )

    # --------------------------------------------------------- data lainnya
    # Kotak "Adik/Kakak" adalah shape ber-textbox, bukan tabel.
    tbs = list(kids[85].iter(W + "txbxContent"))
    set_ptext(tbs[1].findall(W + "p")[0], "{{siblings_groom}}")
    for p in tbs[1].findall(W + "p")[1:]:
        set_ptext(p, "")
    set_ptext(tbs[4].findall(W + "p")[0], "{{siblings_bride}}")
    for p in tbs[4].findall(W + "p")[1:]:
        set_ptext(p, "")
    log.append("data lainnya: 2 placeholder di textbox")

    t3 = kids[88]
    log.append("data lainnya: border disamakan=%d" % normalize_borders(t3))
    row0 = t3.findall(W + "tr")[0]
    for cell, name in zip(row0.findall(W + "tc"),
                          ("menuStall", "menuBuffet", "menuAfterAkad")):
        paras = cell.findall(W + "p")
        heading = paras[0]
        numbered = next((p for p in paras[1:]
                         if p.find(W + "pPr") is not None
                         and p.find(W + "pPr").find(W + "numPr") is not None), paras[1])
        plain = next((p for p in paras[1:]
                      if p.find(W + "pPr") is None
                      or p.find(W + "pPr").find(W + "numPr") is None), paras[-1])
        styled_region(
            cell, heading, name,
            [("heading", heading), ("numbered", numbered), ("plain", plain)],
            remove=paras,
        )
    # baris kedua tabel ini kosong di berkas asli -- ikut dibuang
    for extra in t3.findall(W + "tr")[1:]:
        t3.remove(extra)
    log.append("data lainnya: 3 region menu")

    t4 = kids[91]
    normalize_borders(t4)
    set_ptext(t4.findall(W + "tr")[0].findall(W + "tc")[1].findall(W + "p")[0],
              "{{souvenir_note}}")
    # Baris "Table Cloth" dipindahkan ke tabel catatan ini. Di berkas asli
    # catatannya terselip di DASAR kolom "Makanan After Akad" (row0 cell2 p7-
    # p10), bercampur dengan daftar menu -- sehingga ikut lenyap begitu kolom
    # itu diubah menjadi region menu. Sejajar dengan Souvenir adalah tempat
    # yang memang dimaksud: keduanya catatan, bukan item menu.
    cloth = copy.deepcopy(t4.findall(W + "tr")[0])
    set_ptext(cloth.findall(W + "tc")[0].findall(W + "p")[0], "Table Cloth ")
    set_ptext(cloth.findall(W + "tc")[1].findall(W + "p")[0], "{{table_cloth_note}}")
    t4.append(cloth)
    log.append("data lainnya: baris Table Cloth dipulihkan")

    # --------------------------------------------------------------- makeup
    t5 = kids[96]
    rows5 = t5.findall(W + "tr")
    model5 = rows5[1]
    cells5 = model5.findall(W + "tc")
    set_ptext(cells5[0].findall(W + "p")[0], "{{#makeupRooms}}{{room_label}}")
    for p in cells5[0].findall(W + "p")[1:]:
        set_ptext(p, "")
    detail = cells5[1]
    dparas = detail.findall(W + "p")
    # Empat gaya baris yang benar-benar dipakai berkas asli di sel ini:
    #   heading  -- tebal, tanpa penomoran      (baris 1 kolom keterangan)
    #   numbered -- daftar bernomor             (numId 2)
    #   dash     -- daftar bertanda hubung, miring (numId 17)
    #   note     -- label "Note:", tebal-miring, tanpa penomoran
    # Label "Note:" hanya ada di baris ruangan KEDUA, jadi diambil dari sana.
    second = rows5[2].findall(W + "tc")[1].findall(W + "p")
    dheading = dparas[0]
    dnum = numbered_para(dparas)
    ddash = dash_para(dparas)
    dnote = next((p for p in second
                  if ptext(p).strip().lower().startswith("note")), dparas[0])
    styled_region(
        detail, dheading, "makeupLines",
        [("heading", dheading), ("numbered", dnum), ("dash", ddash), ("note", dnote)],
        remove=dparas,
    )
    tail = detail.findall(W + "p")[-1]
    set_ptext(tail, ptext(tail) + "{{/makeupRooms}}")
    for r in rows5[2:]:
        t5.remove(r)
    log.append("makeup: baris berulang + region makeupLines")

    # -------------------------------------------------- susunan acara (akad)
    # Di berkas asli seksi ini dua tabel terpisah (idx 102 & 105) yang dibuat
    # menyambung secara visual, dengan lebar kolom berbeda tipis. Tabel kedua
    # dibuang: satu tabel yang tumbuh sesuai data justru menghasilkan tampilan
    # yang dimaksud, dan menghilangkan sambungan yang selama ini terlihat.
    t6 = kids[102]
    keep_header_and_one_row(
        t6, 1, "{{#itemsAkad}}", "{{/itemsAkad}}",
        ["{{no_label}}", "{{time_label}}", "{{item}}", "{{pic}}", "{{note}}"],
    )
    body.remove(kids[105])
    log.append("susunan akad: tabel sambungan dibuang, satu tabel berulang")

    # ----------------------------------------------- susunan acara (resepsi)
    keep_header_and_one_row(
        kids[110], 1, "{{#itemsResepsi}}", "{{/itemsResepsi}}",
        ["{{no_label}}", "{{time_label}}", "{{item}}", "{{pic}}", "{{note}}"],
    )

    # --------------------------------------------------------------- layout
    styled_region(
        body, kids[118], "layoutRules",
        [("rule", kids[118])],
        remove=kids[118:121],
    )
    keep_header_and_one_row(
        kids[124], 1, "{{#layoutLegend}}", "{{/layoutLegend}}",
        ["{{number_label}}", "{{content}}"],
    )
    log.append("layout: region aturan + tabel legenda")

    # ------------------------------------------------------------- lampiran
    for i in (139, 143, 148):
        set_ptext(kids[i], "{{couple_title}}")
    keep_header_and_one_row(
        kids[140], 1, "{{#photoGroups}}", "{{/photoGroups}}",
        ["{{number_label}}", "{{group_name}}"],
    )
    keep_header_and_one_row(
        kids[145], 1, "{{#vipGuests}}", "{{/vipGuests}}",
        ["{{number_label}}", "{{full_name}}", "{{position}}"],
    )
    keep_header_and_one_row(
        kids[150], 1, "{{#playlist}}", "{{/playlist}}",
        ["{{number_label}}", "{{title}}", "{{artist}}"],
    )
    styled_region(
        body, kids[152], "playlistNotes",
        [("note", kids[152])],
        remove=kids[152:154],
    )
    log.append("lampiran: 3 tabel berulang + judul + catatan")

    # ------------------------------------------- sorotan draft (Q1) dibuang
    log.append("sorotan warna dibuang: %d" % strip_highlight(body))

    # ---------------------------------------------- border seragam sz=12 (Q1)
    total = sum(normalize_borders(t) for t in body.iter(W + "tbl"))
    log.append("border diseragamkan ke sz=12: %d sisi" % total)

    # --------------------------------------------------------- paginasi
    # Paragraf kosong di dalam satu seksi SENGAJA tidak dipangkas. Ornamen
    # (divider, bingkai, kop) berjangkar pada posisi absolut halaman, jadi
    # begitu konten alir naik beberapa baris, ornamennya menimpa teks. Ini
    # bukan dugaan: percobaan pertama memangkas 26 paragraf dan divider atas
    # halaman VENDORS langsung menindih baris vendor kedua.
    #
    # Risiko yang tersisa -- tabel yang tumbuh mendorong ganjalan ke halaman
    # berikutnya -- jauh lebih kecil, karena tiap seksi sudah dijamin membuka
    # halaman sendiri oleh section break dan page break eksplisit di bawah.
    log.append("section break kembar dirapatkan: %d" % collapse_adjacent_section_breaks(body))
    log.append("halaman kosong di ekor dibuang: %d" % drop_trailing_blank_sections(body))
    added = ensure_section_starts_on_new_page(body)
    log.append("page break eksplisit ditambahkan: %s" % (", ".join(added) or "tidak perlu"))
    sects = sum(1 for p in body if p.tag == W + "p" and has_sect_break(p))
    log.append("section break dipertahankan: %d" % sects)

    # ------------------------------------------------------------- simpan
    out_xml = ET.tostring(root, encoding="UTF-8", xml_declaration=True,
                          standalone=True)
    layout_png = neutral_layout_png(665, 594)
    numbering = num_pool_xml(zin.read("word/numbering.xml").decode("utf-8"))
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    with zipfile.ZipFile(OUT, "w", zipfile.ZIP_DEFLATED) as zout:
        for item in zin.infolist():
            if item.filename == "word/document.xml":
                zout.writestr(item, out_xml)
            elif item.filename == "word/numbering.xml":
                zout.writestr(item, numbering.encode("utf-8"))
            elif item.filename == LAYOUT_IMAGE_ENTRY:
                zout.writestr(item, layout_png)
            else:
                zout.writestr(item, zin.read(item.filename))
    zin.close()
    return log, out_xml


SECTION_TITLES = (
    "VENDORS", "LIST NAMA", "PANITIA KELUARGA", "DATA LAINNYA",
    "LIST & RUANGAN MAKEUP", "SUSUNAN ACARA AKAD DAN RESEPSI",
    "SUSUNAN ACARA RESEPSI", "LAYOUT AKAD", "KETERANGAN LAYOUT AKAD",
    "LIST FOTO TAMU", "LIST TAMU VIP", "PLAYLIST REQUEST LAGU",
)


def ensure_section_starts_on_new_page(body):
    """Pastikan tiap judul seksi membuka halaman baru.

    Dokumen sumber memakai DUA mekanisme sekaligus: 13 section break, dan --
    untuk seksi yang berbagi section dengan seksi sebelumnya -- tumpukan
    paragraf kosong yang kebetulan mendorong judul ke halaman berikutnya.
    Mekanisme kedua itu rapuh: begitu tabel di atasnya tumbuh atau menyusut,
    posisi halamannya ikut bergeser. Judul yang belum didahului section break
    diberi `w:br w:type="page"` eksplisit di awal paragrafnya.
    """
    added = []
    for title in SECTION_TITLES:
        target = next((c for c in body
                       if c.tag == W + "p" and ptext(c).strip() == title), None)
        if target is None:
            raise SystemExit("judul seksi tidak ditemukan: %r" % title)
        idx = list(body).index(target)
        prev = None
        for j in range(idx - 1, -1, -1):
            cand = body[j]
            # Paragraf tanpa teks dilewati -- termasuk yang hanya memuat gambar
            # mengambang (wp:anchor), karena gambar semacam itu tidak menempati
            # aliran teks dan tidak menandai awal halaman.
            if cand.tag == W + "p" and not ptext(cand).strip() \
                    and not has_sect_break(cand):
                continue
            prev = cand
            break
        if prev is not None and prev.tag == W + "p" and has_sect_break(prev):
            continue  # section break sudah membuka halaman baru
        if prev is None:
            continue  # judul pertama dokumen
        r = ET.Element(W + "r")
        br = ET.SubElement(r, W + "br")
        br.set(W + "type", "page")
        pr = target.find(W + "pPr")
        target.insert(1 if pr is not None else 0, r)
        added.append(title)
    return added


def collapse_adjacent_section_breaks(body):
    """Dua section break beruntun tanpa isi = satu halaman kosong.

    Muncul mis. di bekas tabel sambungan SUSUNAN ACARA yang dibuang: section
    break miliknya tertinggal tepat di sebelah section break berikutnya.
    """
    removed = 0
    prev_break = False
    for child in list(body):
        if child.tag != W + "p":
            prev_break = False
            continue
        empty = not ptext(child).strip() and child.find(".//" + W + "drawing") is None
        if empty and has_sect_break(child):
            if prev_break:
                body.remove(child)
                removed += 1
                continue
            prev_break = True
        elif not empty:
            prev_break = False
    return removed


def drop_trailing_blank_sections(body):
    """Buang section kosong beruntun di ekor dokumen.

    Berkas asli berakhir dengan 4 halaman kosong -- murni sisa section break
    yang tidak membawa isi apa pun. Termasuk cacat kosmetik yang boleh
    dirapikan (Q1), dan kalau dibiarkan setiap rundown yang dicetak WO ikut
    membawa 4 lembar kosong.
    """
    removed = 0
    for child in reversed(list(body)):
        if child.tag == W + "sectPr":
            continue  # sectPr milik body -- penutup dokumen, bukan halaman kosong
        if child.tag != W + "p":
            break
        if ptext(child).strip() or child.find(".//" + W + "drawing") is not None:
            break
        body.remove(child)
        removed += 1
    return removed




if __name__ == "__main__":
    log, xml = build()
    for line in log:
        print("  -", line)
    leftovers = xml.count(b"DINDA") + xml.count(b"REZA") + xml.count(b"Samala")
    print("\ntemplate:", os.path.relpath(OUT, os.path.join(HERE, "..", "..")))
    print("ukuran document.xml:", len(xml), "byte")
    print("sisa nama pasangan asli di XML:", leftovers)

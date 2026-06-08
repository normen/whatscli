#!/usr/bin/env python3
"""Gera um recurso COFF (.syso) embutindo um .ico no executavel Windows.

Saida: zapterm_windows_amd64.syso na raiz do repo. O sufixo _windows_amd64
faz o 'go build' usar este arquivo SOMENTE para a build windows/amd64,
sem afetar a build Linux.
"""
import os
import struct

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ICO = os.path.join(ROOT, "assets", "zapterm.ico")
OUT = os.path.join(ROOT, "zapterm_windows_amd64.syso")

RT_ICON = 3
RT_GROUP_ICON = 14
LANG = 0

# --- 1. ler o .ico ---------------------------------------------------------
with open(ICO, "rb") as f:
    ico = f.read()

reserved, itype, count = struct.unpack_from("<HHH", ico, 0)
assert reserved == 0 and itype == 1, "arquivo .ico invalido"

entries = []   # (header12bytes, image_bytes)
for i in range(count):
    base = 6 + i * 16
    w, h, cc, res, planes, bits, size, offset = struct.unpack_from("<BBBBHHII", ico, base)
    hdr12 = struct.pack("<BBBBHHI", w, h, cc, res, planes, bits, size)  # 12 bytes
    img = ico[offset:offset + size]
    entries.append((hdr12, img))

N = len(entries)

# --- 2. montar os blocos de dados dos recursos -----------------------------
# GRPICONDIR: header(6) + N * GRPICONDIRENTRY(14)
grp = struct.pack("<HHH", 0, 1, N)
for i, (hdr12, img) in enumerate(entries):
    grp += hdr12 + struct.pack("<H", i + 1)   # nId = i+1

def align4(n):
    return (n + 3) & ~3

# --- 3. calcular offsets da secao .rsrc ------------------------------------
DIR = 16        # IMAGE_RESOURCE_DIRECTORY
ENT = 8         # IMAGE_RESOURCE_DIRECTORY_ENTRY
DE = 16         # IMAGE_RESOURCE_DATA_ENTRY

off_root = 0
off_icon_dir = off_root + DIR + 2 * ENT            # root tem 2 tipos
off_group_dir = off_icon_dir + DIR + N * ENT
off_lang_icon = off_group_dir + DIR + 1 * ENT      # lang dirs dos RT_ICON
# cada lang dir = DIR + 1*ENT
def lang_icon(i):
    return off_lang_icon + i * (DIR + ENT)
off_lang_group = off_lang_icon + N * (DIR + ENT)
off_de = off_lang_group + (DIR + ENT)              # data entries
def de_icon(i):
    return off_de + i * DE
off_de_group = off_de + N * DE
off_data = off_de + (N + 1) * DE                   # dados brutos

# posicoes dos dados brutos (alinhados a 4)
pos = off_data
img_pos = []
for (_, img) in entries:
    img_pos.append(pos)
    pos = align4(pos + len(img))
grp_pos = pos
pos = align4(pos + len(grp))
rsrc_size = pos

# --- 4. emitir a secao .rsrc ----------------------------------------------
buf = bytearray(rsrc_size)
relocs = []   # (offset_do_campo, addend) -> ADDR32NB contra simbolo .rsrc

def put(off, data):
    buf[off:off + len(data)] = data

def res_dir(off, named, id_entries):
    # id_entries: lista de (id, child_offset, is_dir)
    put(off, struct.pack("<IIHHHH", 0, 0, 0, 0, named, len(id_entries)))
    p = off + DIR
    for (rid, child, is_dir) in id_entries:
        val = child | (0x80000000 if is_dir else 0)
        put(p, struct.pack("<II", rid, val))
        p += ENT

# root: tipos RT_ICON(3) e RT_GROUP_ICON(14), ordem crescente
res_dir(off_root, 0, [(RT_ICON, off_icon_dir, True),
                      (RT_GROUP_ICON, off_group_dir, True)])

# diretorio RT_ICON: ids 1..N
res_dir(off_icon_dir, 0, [(i + 1, lang_icon(i), True) for i in range(N)])

# diretorio RT_GROUP_ICON: id 1
res_dir(off_group_dir, 0, [(1, off_lang_group, True)])

# lang dirs RT_ICON -> data entry
for i in range(N):
    res_dir(lang_icon(i), 0, [(LANG, de_icon(i), False)])
# lang dir RT_GROUP_ICON -> data entry
res_dir(off_lang_group, 0, [(LANG, off_de_group, False)])

# data entries (OffsetToData = offset dentro da secao; precisa de reloc)
for i, (_, img) in enumerate(entries):
    de = de_icon(i)
    put(de, struct.pack("<IIII", img_pos[i], len(img), 0, 0))
    relocs.append((de, img_pos[i]))   # campo OffsetToData
# data entry do grupo
put(off_de_group, struct.pack("<IIII", grp_pos, len(grp), 0, 0))
relocs.append((off_de_group, grp_pos))

# dados brutos
for (_, img), p in zip(entries, img_pos):
    put(p, img)
put(grp_pos, grp)

# --- 5. montar o arquivo COFF ---------------------------------------------
IMAGE_FILE_MACHINE_AMD64 = 0x8664
IMAGE_SCN = 0x40000040           # CNT_INITIALIZED_DATA | MEM_READ
IMAGE_REL_AMD64_ADDR32NB = 0x0003
IMAGE_SYM_CLASS_STATIC = 3

num_relocs = len(relocs)
ptr_rawdata = 20 + 40            # file header + 1 section header
ptr_relocs = ptr_rawdata + rsrc_size
ptr_symtab = ptr_relocs + num_relocs * 10
num_syms = 1

out = bytearray()
# COFF file header
out += struct.pack("<HHIIIHH",
                   IMAGE_FILE_MACHINE_AMD64, 1, 0,
                   ptr_symtab, num_syms, 0, 0)
# section header .rsrc
name = b".rsrc\0\0\0"
out += name
out += struct.pack("<IIIIIIHH",
                   0,            # VirtualSize
                   0,            # VirtualAddress
                   rsrc_size,    # SizeOfRawData
                   ptr_rawdata,  # PointerToRawData
                   ptr_relocs,   # PointerToRelocations
                   0,            # PointerToLinenumbers
                   num_relocs, 0)
out += struct.pack("<I", IMAGE_SCN)
# raw data
out += buf
# relocations (cada: VirtualAddress, SymbolIndex=0, Type)
for (field_off, _addend) in relocs:
    out += struct.pack("<IIH", field_off, 0, IMAGE_REL_AMD64_ADDR32NB)
# symbol table: 1 simbolo ".rsrc"
out += b".rsrc\0\0\0"                        # name (8 bytes)
out += struct.pack("<IhHBB", 0, 1, 0, IMAGE_SYM_CLASS_STATIC, 0)
# string table (vazia) = tamanho 4
out += struct.pack("<I", 4)

with open(OUT, "wb") as f:
    f.write(out)

print("Gerado:", OUT, "(", len(out), "bytes,", N, "imagens )")

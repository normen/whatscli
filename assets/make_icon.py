#!/usr/bin/env python3
"""Gera o icone do ZapTerm: balao de chat verde com prompt de terminal '>_'."""
import os
from PIL import Image, ImageDraw

OUT_DIR = os.path.dirname(os.path.abspath(__file__))

# Cores
GREEN = (37, 211, 102, 255)       # verde "WhatsApp"
GREEN_DARK = (18, 140, 70, 255)    # contorno/sombra
WHITE = (255, 255, 255, 255)

def rounded_rect(draw, box, radius, fill):
    draw.rounded_rectangle(box, radius=radius, fill=fill)

def make(size):
    # Desenha grande e reduz (anti-aliasing)
    S = size * 8
    img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)

    m = int(S * 0.10)                       # margem
    bub = (m, m, S - m, int(S * 0.80))      # corpo do balao
    r = int(S * 0.20)

    # sombra leve
    d.rounded_rectangle((bub[0]+S*0.02, bub[1]+S*0.03, bub[2]+S*0.02, bub[3]+S*0.03),
                        radius=r, fill=(0, 0, 0, 60))
    # corpo do balao
    rounded_rect(d, bub, r, GREEN)

    # rabinho do balao (canto inferior esquerdo)
    tail = [(int(S*0.22), int(bub[3]-S*0.02)),
            (int(S*0.12), int(S*0.92)),
            (int(S*0.40), int(bub[3]-S*0.02))]
    d.polygon(tail, fill=GREEN)

    # Prompt ">" desenhado como chevron
    cx, cy = int(S*0.40), int(S*0.45)
    lw = max(2, int(S*0.045))
    d.line([(int(S*0.28), int(S*0.33)), (cx, cy)], fill=WHITE, width=lw)
    d.line([(cx, cy), (int(S*0.28), int(S*0.57))], fill=WHITE, width=lw)

    # cursor "_" (underline)
    d.line([(int(S*0.50), int(S*0.57)), (int(S*0.72), int(S*0.57))],
           fill=WHITE, width=lw)

    return img.resize((size, size), Image.LANCZOS)

sizes = [16, 24, 32, 48, 64, 128, 256]
imgs = [make(s) for s in sizes]

# salva PNG 256 para preview e o ICO multi-resolucao
# (usa o MAIOR como base; PIL nao faz upscale, entao a base precisa ser 256)
imgs[-1].save(os.path.join(OUT_DIR, "zapterm.png"))
imgs[-1].save(os.path.join(OUT_DIR, "zapterm.ico"),
             sizes=[(s, s) for s in sizes])
print("Gerado:", os.path.join(OUT_DIR, "zapterm.ico"))
print("Gerado:", os.path.join(OUT_DIR, "zapterm.png"))

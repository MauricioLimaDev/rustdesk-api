#!/usr/bin/env python3
"""Contagem dos downloads do Suporte Appelsoft (rodar na VPS, em ~/rustdesk).

    python3 contar-downloads.py                 # tudo
    python3 contar-downloads.py --desde 2026-12-01
    python3 contar-downloads.py --mes           # também por mês

Lê:
  caddy-data/downloads.log (+ os antigos downloads-*.log e .log.gz) - log do Caddy
  downloads/publicacoes.log - "<data> <plataforma/canal> <versao>", gravado a cada publicação

Origem de cada download:
  link        - link público (/download/cliente, /download/cliente-x86)
  painel      - botão Baixar do painel (com login)
  atualizacao - atualizador automático (PowerShell no cliente; caminho com chave na Equipe)
A versão é a que estava publicada no momento do download (pelo publicacoes.log).
Downloads em pedaços (Range) só contam o pedaço inicial; respostas de erro não contam.
"""
import argparse
import bisect
import glob
import gzip
import json
import os
import re
from collections import Counter, defaultdict
from datetime import datetime

CANAL_PUBLICACAO = {
    "cliente": "windows/cliente",
    "cliente-x86": "windows/cliente-x86",
    "equipe": "windows/equipe",
    "equipe-mac-arm64": "mac/equipe",
}
RE_EQUIPE_ATUALIZACAO = re.compile(r"^/download/a/[A-Za-z0-9]{32,}/equipe\.exe$")
RE_PAINEL = re.compile(r"^/api/admin/download/arquivo/([a-z0-9-]+)$")
CHECAGEM = {"/download/versao.json": "cliente", "/download/versao-x86.json": "cliente-x86"}


def linhas_log(pasta):
    arquivos = sorted(glob.glob(os.path.join(pasta, "downloads-*.log*")))
    atual = os.path.join(pasta, "downloads.log")
    if os.path.exists(atual):
        arquivos.append(atual)
    for arq in arquivos:
        abrir = gzip.open if arq.endswith(".gz") else open
        with abrir(arq, "rt", encoding="utf-8", errors="replace") as f:
            for linha in f:
                try:
                    yield json.loads(linha)
                except ValueError:
                    continue


def ler_publicacoes(arq):
    pub = defaultdict(list)  # canal -> [(ts, versao)]
    if not os.path.exists(arq):
        return pub
    with open(arq, encoding="utf-8") as f:
        for linha in f:
            partes = linha.split()
            if len(partes) != 3:
                continue
            try:
                ts = datetime.fromisoformat(partes[0]).timestamp()
            except ValueError:
                continue
            pub[partes[1]].append((ts, partes[2]))
    for lista in pub.values():
        lista.sort()
    return pub


def versao_em(pub, tipo, ts):
    lista = pub.get(CANAL_PUBLICACAO.get(tipo, ""), [])
    i = bisect.bisect_right([t for t, _ in lista], ts)
    return lista[i - 1][1] if i else "(antes do registro)"


def classificar(path, ua):
    if path == "/download/cliente":
        return "cliente", "atualizacao" if "WindowsPowerShell" in ua else "link"
    if path == "/download/cliente-x86":
        return "cliente-x86", "atualizacao" if "WindowsPowerShell" in ua else "link"
    if RE_EQUIPE_ATUALIZACAO.match(path):
        return "equipe", "atualizacao"
    m = RE_PAINEL.match(path)
    if m:
        return m.group(1), "painel"
    return None, None


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--logs", default="caddy-data", help="pasta do downloads.log (padrão: caddy-data)")
    ap.add_argument("--publicacoes", default="downloads/publicacoes.log")
    ap.add_argument("--desde", help="AAAA-MM-DD")
    ap.add_argument("--mes", action="store_true", help="mostrar também por mês")
    a = ap.parse_args()
    desde = datetime.fromisoformat(a.desde).timestamp() if a.desde else 0

    pub = ler_publicacoes(a.publicacoes)
    total = Counter()           # (tipo, versao, origem)
    por_mes = Counter()         # (mes, tipo, origem)
    checagens = defaultdict(set)  # (dia, canal) -> IPs que consultaram a versão

    for e in linhas_log(a.logs):
        ts = e.get("ts", 0)
        req = e.get("request") or {}
        if ts < desde or req.get("method") != "GET":
            continue
        path = (req.get("uri") or "").split("?")[0]
        status = e.get("status", 0)
        ip = req.get("client_ip") or req.get("remote_ip") or ""
        if path in CHECAGEM and status == 200:
            dia = datetime.fromtimestamp(ts).strftime("%Y-%m-%d")
            checagens[(dia, CHECAGEM[path])].add(ip)
            continue
        headers = req.get("headers") or {}
        ua = " ".join(headers.get("User-Agent") or [])
        rng = " ".join(headers.get("Range") or [])
        if status not in (200, 206) or (rng and not rng.replace(" ", "").startswith("bytes=0-")):
            continue
        tipo, origem = classificar(path, ua)
        if not tipo:
            continue
        total[(tipo, versao_em(pub, tipo, ts), origem)] += 1
        por_mes[(datetime.fromtimestamp(ts).strftime("%Y-%m"), tipo, origem)] += 1

    if not total and not checagens:
        print("Nenhum download registrado ainda.")
        return

    print(f"{'Versão do app':<18} {'Número':<22} {'Origem':<12} {'Downloads':>9}")
    for (tipo, versao, origem), n in sorted(total.items()):
        print(f"{tipo:<18} {versao:<22} {origem:<12} {n:>9}")
    por_tipo = Counter()
    for (tipo, _, _), n in total.items():
        por_tipo[tipo] += n
    print("\nTotal por versão do app: " + ", ".join(f"{t} {n}" for t, n in sorted(por_tipo.items())))

    if a.mes:
        print(f"\n{'Mês':<9} {'Versão do app':<18} {'Origem':<12} {'Downloads':>9}")
        for (mes, tipo, origem), n in sorted(por_mes.items()):
            print(f"{mes:<9} {tipo:<18} {origem:<12} {n:>9}")

    if checagens:
        print("\nMáquinas que consultaram atualização (IPs distintos por dia, últimos 7 dias com registro):")
        dias = sorted({d for d, _ in checagens})[-7:]
        for dia in dias:
            partes = [f"{c} {len(checagens[(dia, c)])}" for c in ("cliente", "cliente-x86") if (dia, c) in checagens]
            print(f"  {dia}: " + ", ".join(partes))


if __name__ == "__main__":
    main()

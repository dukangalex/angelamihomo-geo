#!/usr/bin/env python3
"""Extract merged (country, region, city) ranges from the official
DB-IP City Lite MMDB. Adjacent ranges with identical tuples are
merged on the fly. Output: ranges.bin (binary, see encoder step)
plus printed statistics."""
import maxminddb, ipaddress, struct, sys, time

SRC = sys.argv[1] if len(sys.argv) > 1 else 'dbip-city-lite-2026-11.mmdb'
OUT = 'ranges.bin'

def tup(rec):
    if not rec:
        return None
    country = rec.get('country', {}).get('iso_code', '')
    city = rec.get('city', {}).get('names', {}).get('en', '')
    subs = rec.get('subdivisions') or []
    region = subs[0].get('names', {}).get('en', '') if subs else ''
    if not country:
        return None
    return (country, region, city)

def main():
    r = maxminddb.open_database(SRC)
    t0 = time.time()
    n_raw = 0
    dict_map = {}
    dict_list = []
    # per-family current open range: [start_int, end_int, tuple_id]
    cur = {4: None, 6: None}
    counts = {4: 0, 6: 0}
    fout = open(OUT, 'wb')
    # record format: family(1B) start(16B big-endian) end(16B) tuple_id(4B)
    def flush(fam):
        c = cur[fam]
        if c is None:
            return
        fout.write(struct.pack('>B16s16sI', fam, c[0].to_bytes(16, 'big'), c[1].to_bytes(16, 'big'), c[2]))
        counts[fam] += 1
        cur[fam] = None
    for network, rec in r:
        n_raw += 1
        ip = network.network_address
        # classify: ipv4-mapped ipv6 networks count as v4
        if isinstance(ip, ipaddress.IPv6Address) and ip.ipv4_mapped is not None:
            fam = 4
            start = int(ip.ipv4_mapped)
            end = int(network.broadcast_address.ipv4_mapped)
        elif isinstance(ip, ipaddress.IPv4Address):
            fam = 4
            start = int(ip)
            end = int(network.broadcast_address)
        else:
            fam = 6
            start = int(ip)
            end = int(network.broadcast_address)
        t = tup(rec)
        if t is None:
            flush(fam)
            continue
        tid = dict_map.get(t)
        if tid is None:
            tid = len(dict_list)
            dict_map[t] = tid
            dict_list.append(t)
        c = cur[fam]
        if c is not None and c[2] == tid and c[1] + 1 == start:
            c[1] = end
        else:
            flush(fam)
            cur[fam] = [start, end, tid]
    for fam in (4, 6):
        flush(fam)
    fout.close()
    import json
    with open('dict.json', 'w', encoding='utf-8') as f:
        json.dump(dict_list, f, ensure_ascii=False)
    print(f'raw networks: {n_raw}')
    print(f'merged ranges: v4={counts[4]} v6={counts[6]} total={counts[4]+counts[6]}')
    print(f'unique (country,region,city) tuples: {len(dict_list)}')
    print(f'elapsed: {time.time()-t0:.0f}s')

if __name__ == '__main__':
    main()

#!/bin/sh
# 一键构建 amcity.bin：下载 DB-IP 官方当月 Lite 库 -> 提取合并 -> 紧凑编码
# 用法：./build.sh [YYYY-MM]（缺省为当前月）
set -eu
MONTH="${1:-$(date +%Y-%m)}"
WORK="$(cd "$(dirname "$0")" && pwd)/.build"
mkdir -p "$WORK"
MMDB_GZ="$WORK/dbip-city-lite-$MONTH.mmdb.gz"
MMDB="$WORK/dbip-city-lite-$MONTH.mmdb"
if [ ! -f "$MMDB" ]; then
  curl -fL "https://download.db-ip.com/free/dbip-city-lite-$MONTH.mmdb.gz" -o "$MMDB_GZ"
  gzip -dc "$MMDB_GZ" > "$MMDB"
  echo "已下载。校验值（请与 https://db-ip.com/db/download/ip-to-city-lite 官方公布值核对）："
  md5sum "$MMDB"; sha1sum "$MMDB"
fi
cd "$WORK"
if [ ! -d .venv ]; then
  python3 -m venv .venv
  .venv/bin/pip install -q maxminddb
fi
cp "$(dirname "$0")/extract.py" .
.venv/bin/python extract.py "dbip-city-lite-$MONTH.mmdb"
cp "$(dirname "$0")/encoder.go" "$(dirname "$0")/go.mod" .
GOFLAGS=-mod=mod go run encoder.go
gzip -9 -k -f amcity.bin
sha256sum amcity.bin amcity.bin.gz | tee sha256sums.txt
echo "产物：$WORK/amcity.bin / amcity.bin.gz / sha256sums.txt"

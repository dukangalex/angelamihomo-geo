# AngelaMihomo Geo Data（城市级 IP 地理数据仓）

AngelaMihomo 的自有地理数据仓。本仓发布城市级 IP 地理数据文件，App 在首次启动时从本仓 Release 自动拉取、校验后落到本地，之后所有地理查询都在本地完成——运行时不依赖任何第三方在线地理服务。

## 数据来源与授权

- 数据源：DB-IP 官方发布的 **IP to City Lite** 数据库（月度更新），官方下载页：https://db-ip.com/db/download/ip-to-city-lite
- 授权：Creative Commons Attribution 4.0 International（CC BY 4.0）
- 署名：**IP Geolocation by [DB-IP](https://db-ip.com)**
- 本仓对原始数据只做字段裁剪（仅保留国家、省份/州、城市）与紧凑格式重编码，不改变数据内容含义；个别网格单元内存在多个城市记录时取覆盖份额最大者，详见 FORMAT.md。
- 本仓构建脚本以 GPL-3.0 发布；数据文件权利归 DB-IP 所有、按 CC BY 4.0 使用。

## 数据文件

每个 Release 附带：

| 文件 | 说明 |
|---|---|
| `amcity.bin` | 城市库本体（格式见 FORMAT.md），由 App 解压后直接使用 |
| `amcity.bin.gz` | 同上内容的 gzip 压缩版，App 实际下载此文件（约 10 MB） |
| `sha256sums.txt` | 上述两文件的 SHA-256 校验值 |

当前版本（v2026.11.1，源数据为 DB-IP 2026-11 版）：`amcity.bin` 29,894,012 字节，覆盖 IPv4 /24 网格 14,461,948 个单元与 IPv6 /48 网格，城市/国家与源数据的一致率经抽样实测为 99.8%（网格多数决单元除外）。

## 版本与更新节奏

- Release 命名：`v<年>.<月>.<序号>`，与源数据月份对应。
- 本仓随 AngelaMihomo 项目的既有维护节奏同步重建与发布（源数据为月度更新），不单独长期搁置。

## 构建

```sh
./build.sh 2026-11   # 参数为源数据月份，缺省取当前月
```

流程：从 DB-IP 官方直链下载当月 MMDB → 与官方公布的校验值核对 → `extract.py` 提取并合并区间 → `encoder.go` 编码为 AMCITY 格式。依赖：Python 3（包 `maxminddb`）、Go。

格式规范见 [FORMAT.md](FORMAT.md)。

---

## English

Self-hosted city-level IP geolocation data for **AngelaMihomo**. The app downloads the data file from this repository's Releases on first launch, verifies its SHA-256, stores it locally, and answers all geolocation lookups offline — no third-party online geolocation service is contacted at runtime.

- Source: **IP to City Lite** by DB-IP (official download: https://db-ip.com/db/download/ip-to-city-lite), licensed under CC BY 4.0. Attribution: **IP Geolocation by [DB-IP](https://db-ip.com)**.
- Processing: fields are trimmed to country / region / city and re-encoded into the compact AMCITY format (see FORMAT.md). Within a grid cell that contains multiple city records, the record covering the largest share wins.
- Releases are named `v<year>.<month>.<n>` and are rebuilt in step with the AngelaMihomo project's maintenance cadence.
- Build scripts in this repository are GPL-3.0; the data remains © DB-IP under CC BY 4.0.

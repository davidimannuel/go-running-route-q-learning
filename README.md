# Go Running Route Q-Learning

Penulisan ulang core TA **Sistem Rekomendasi Rute Lari Menggunakan Algoritma Q-Learning Berbasis Data OpenStreetMap di Daerah Dukuh Kupang, Surabaya** ke Go. Target penelitian adalah rute loop 5 km dengan toleransi 10%.

Go menangani pengambilan data dan pembentukan graf. Python membaca CSV keluaran Go untuk analisis, tabel, dan diagram melalui JupyterLab. Setiap percobaan dikelompokkan dalam satu **workdir** yang berisi konfigurasi, cache, dan hasil.

## Status implementasi

| Komponen | Status |
|---|---|
| CLI `download` | Pengambilan OSM melalui Overpass, cache, dan penyimpanan JSON |
| CLI `analyze` | Membaca snapshot lokal, membentuk graf mentah, dan mengekspor CSV |
| Graf | Edge terarah dua arah, edge paralel, asal way, tag asli, dan panjang Haversine |
| Python | Environment analisis dengan uv, pandas, matplotlib, PyYAML, dan JupyterLab |
| Notebook analisis | `analisa.ipynb`: konsistensi CSV, ringkasan graf, panjang segmen, kelengkapan/distribusi empat atribut, dan kualitas width |
| Q-learning dan training Go | Belum diimplementasikan |
| Peta interaktif | Ditunda sampai tahap visualisasi peta |

Graf saat ini belum melalui simplifikasi, pemilihan komponen terhubung, atau pemotongan batas wilayah seperti alur akhir OSMnx. Setiap pasangan node berurutan pada way menghasilkan edge maju dan balik. CSV mencatat **edge terarah**, sehingga jumlah edge bukan jumlah ruas fisik unik.

## Struktur repository

```text
cmd/cli/             Entry point, command download/analyze, dan penulisan hasil
internal/config/     Pembacaan dan validasi konfigurasi YAML
internal/logger/     Structured logging berbasis log/slog
internal/osm/        Overpass, cache, graf, jarak, pembacaan JSON, dan ekspor CSV
workdirs/            Konfigurasi dan artefak masing-masing percobaan
analisa.ipynb        Analisis CSV dan diagram untuk graf mentah
pyproject.toml       Dependensi analisis Python dan batas versi interpreter
uv.lock              Versi pasti dependensi Python
.python-version      Pilihan versi Python untuk uv
```

Pengujian Go berada bersama package terkait dalam berkas `*_test.go`. Notebook utama tersedia pada [analisa.ipynb](analisa.ipynb).

## Prasyarat

- **Go 1.27.2 atau lebih baru**, mengikuti deklarasi dalam `go.mod`.
- **uv** untuk mengelola Python, `.venv`, dan dependensi.
- **Python 3.13.x**, yang dapat dipasang oleh uv.
- Koneksi internet untuk setup dependensi dan pengambilan OSM pertama.

Periksa instalasi:

```bash
go version
uv --version
```

Pada macOS dengan Homebrew, uv dapat dipasang menggunakan `brew install uv`. Pilihan platform lain tersedia di [panduan instalasi uv](https://docs.astral.sh/uv/getting-started/installation/). Installer Go tersedia di [situs resmi Go](https://go.dev/dl/).

## Setup awal

Jalankan dari root repository:

```bash
go mod download
uv python install 3.13
uv sync --locked
```

`uv sync --locked` membuat `.venv` lokal dan memasang dependensi sesuai `uv.lock`, termasuk grup `dev`. Tidak perlu menjalankan `uv init` atau mengaktifkan environment dengan `source` setiap sesi. `uv run --locked` menggunakan environment proyek secara otomatis.

`.python-version` memilih seri 3.13; `requires-python = ">=3.13,<3.14"` membatasi proyek ke seri tersebut. Versi patch interpreter tidak dikunci oleh pin minor ini. Periksa versi lengkap saat mencatat hasil analisis:

```bash
uv run --locked python --version
```

Daftarkan kernel lokal untuk membedakannya dari environment proyek TA Python lama:

```bash
uv run --locked python -m ipykernel install --sys-prefix \
  --name go-running-route-analysis \
  --display-name "Running Route Analysis (Python 3.13)"
```

Kernel disimpan di `.venv` proyek. Jalankan lagi perintah pendaftaran jika environment dibuat ulang.

## Konfigurasi dan workdir

Workdir adalah folder yang langsung memuat `config.yaml`. Contoh yang tersedia menggunakan nama `dukuh-kupang`; nama workdir bebas dan dapat diganti sesuai percobaan.

```text
workdirs/dukuh-kupang/
  config.yaml
  cache/                 Dibuat saat download
  output/
    osm.json             Snapshot hasil download
    nodes.csv            Tabel node hasil analyze
    edges.csv            Tabel edge hasil analyze
```

Tidak ada subfolder `input`. Contoh `config.yaml`:

```yaml
schema_version: 1

map:
  latitude: -7.282187
  longitude: 112.709282
  radius_m: 3025
  timeout_sec: 90

output_dir: output
cache_dir: cache
```

| Parameter | Fungsi |
|---|---|
| `schema_version` | Wajib bernilai `1` |
| `map.latitude`, `map.longitude` | Koordinat pusat pengambilan data, dalam derajat |
| `map.radius_m` | Jarak pusat ke sisi bbox, dalam meter; bukan diameter atau radius filter lingkaran |
| `map.timeout_sec` | Batas waktu pemrosesan query Overpass, dalam detik |
| `output_dir` | Direktori hasil relatif terhadap workdir; default `output` |
| `cache_dir` | Direktori cache relatif terhadap workdir; default `cache` |

Parameter `map` wajib diisi. Path direktori harus relatif dan tetap berada di dalam workdir. Field YAML yang tidak dikenal ditolak. Konfigurasi tidak memerlukan opsi ekspor: `analyze` selalu menulis kedua CSV.

## Menjalankan CLI Go

Semua contoh berikut dijalankan dari root repository.

### Mengambil snapshot OSM

```bash
go run ./cmd/cli download --workdir workdirs/dukuh-kupang
```

Command membaca konfigurasi, menggunakan query bbox dengan filter walking, kemudian menyimpan `osm.json` ke `output_dir`. Snapshot berisi data OSM mentah, bukan graf OSMnx yang sudah diproses.

Cache diidentifikasi oleh hash endpoint dan query, termasuk parameter lokasi serta timeout. Cache valid digunakan kembali tanpa request HTTP. Cache rusak diunduh ulang; cache tidak memiliki masa kedaluwarsa otomatis. Untuk meminta data baru secara sengaja:

```bash
go run ./cmd/cli download --workdir workdirs/dukuh-kupang --force
```

`--force` melewati pembacaan cache dan memperbaruinya setelah pengambilan berhasil. Mengulang `download` menimpa `osm.json`; jalankan `analyze` sesudahnya agar CSV sesuai snapshot terbaru. Data OSM dapat berubah antarunduhan, sehingga simpan salinan snapshot untuk perbandingan penelitian yang memerlukan data identik.

### Membentuk graf dan mengekspor CSV

```bash
go run ./cmd/cli analyze --workdir workdirs/dukuh-kupang
```

Command membaca `output_dir/osm.json` tanpa menghubungi Overpass, membentuk graf mentah, lalu selalu menulis atau menimpa `nodes.csv` dan `edges.csv` di direktori yang sama. Snapshot sumber tetap dipertahankan. Jika JSON belum tersedia, jalankan `download` terlebih dahulu.

| File | Kolom |
|---|---|
| `nodes.csv` | `node_id`, `latitude`, `longitude` |
| `edges.csv` | `from`, `to`, `key`, `way_id`, `length_m`, `name`, `highway`, `surface`, `width`, `access` |

Node diurutkan berdasarkan ID dan edge berdasarkan `(from, to, key)`. `way_id` menghubungkan segmen ke way asal. Key saat ini merupakan indeks edge keluar per node sumber. Edge arah balik dan paralel tetap menjadi baris terpisah.

`length_m` dihitung per segmen dengan Haversine menggunakan radius bumi 6.371.009 meter dan disimpan tanpa pembulatan. Tag `width` diekspor sebagai teks asli, sehingga kosong, `3`, `3 m`, dan `3;4` tetap dapat dibedakan. Empat atribut analisis adalah **highway, surface, width, dan access**.

Setiap file ditulis melalui file sementara sebelum mengganti hasil lama. Penggantian berlaku per file, bukan satu transaksi untuk seluruh CSV. Gunakan hasil setelah command selesai dengan sukses.

### Log, bantuan, dan binary

Default log adalah level `INFO` ke atas, berbentuk JSON melalui stderr. Tambahkan `--debug` untuk detail konfigurasi, cache, dan request:

```bash
go run ./cmd/cli analyze --workdir workdirs/dukuh-kupang --debug
go run ./cmd/cli --help
```

Untuk membuat binary lokal:

```bash
mkdir -p bin
go build -o bin/running-route ./cmd/cli
./bin/running-route analyze --workdir workdirs/dukuh-kupang
```

## Menjalankan analisis Python

Dependensi langsung analisis:

| Paket | Peran |
|---|---|
| pandas | Membaca CSV dan mengolah tabel |
| matplotlib | Membuat diagram dan menyimpan gambar |
| PyYAML | Membaca `output_dir` dari konfigurasi workdir |
| JupyterLab, ipykernel | Editor notebook dan kernel Python, dalam grup `dev` |

Mulai sesi dari root repository:

```bash
uv sync --locked
uv run --locked jupyter lab analisa.ipynb
```

Buka [analisa.ipynb](analisa.ipynb), ubah `WORKDIR` pada cell awal, lalu jalankan cell berurutan. Hasil tabel dan diagram disimpan ke `output_dir/analysis/` dalam workdir tersebut. Notebook tidak melakukan unduhan atau training.

Pilih kernel **Running Route Analysis (Python 3.13)**. Sebelum analisis, periksa environment melalui cell:

```python
import sys
from pathlib import Path

print(sys.version)
print(sys.executable)
print(Path.cwd())
```

Executable harus menunjuk ke `.venv` repository ini. Folder kerja notebook harus sesuai path data yang dipakai. Mengubah direktori pada terminal JupyterLab tidak otomatis mengubah folder kerja kernel notebook.

Contoh membaca CSV untuk notebook yang disimpan dan dijalankan di root repository:

```python
from pathlib import Path

import pandas as pd
import yaml

workdir = Path("workdirs/dukuh-kupang")
config = yaml.safe_load((workdir / "config.yaml").read_text())
output_dir = workdir / config.get("output_dir", "output")

nodes = pd.read_csv(
    output_dir / "nodes.csv",
    dtype={"node_id": "int64"},
)
edges = pd.read_csv(
    output_dir / "edges.csv",
    dtype={
        "from": "int64", "to": "int64", "key": "int64", "way_id": "int64",
        "name": "string", "highway": "string", "surface": "string",
        "width": "string", "access": "string",
    },
    keep_default_na=False,
)

print(f"Nodes: {len(nodes):,}; directed edges: {len(edges):,}")
```

`keep_default_na=False` mempertahankan tag kosong sebagai string kosong. Pembacaan `width` sebagai teks menjaga nilai asal untuk pemeriksaan dan pengelompokan berikutnya. Notebook membaca CSV yang dihasilkan Go; aturan pembentukan graf tetap berada di Go.

Untuk menjalankan file Python yang sudah dibuat, ganti path contoh berikut dengan file tersebut:

```bash
uv run --locked python path/to/analysis.py
```

Sebelum menyimpan hasil final, restart kernel dan jalankan semua cell dari atas. Setelah CSV diperbarui oleh CLI, jalankan ulang cell pembacaan agar DataFrame di memori ikut diperbarui. Buat tabel dan grafik berdasarkan satu snapshot yang konsisten.

## Mengelola dependensi Python

Tambahkan paket dari terminal proyek menggunakan uv:

```bash
uv add nama-paket
uv add --dev nama-alat
```

Perintah tersebut memperbarui `pyproject.toml`, `uv.lock`, dan environment. Tinjau perubahan, lalu restart kernel sebelum memakai versi paket baru. Jika JupyterLab atau ipykernel diperbarui, tutup server dan buka kembali dari terminal.

Untuk menghapus paket atau melakukan upgrade yang disengaja:

```bash
uv remove nama-paket
uv lock --upgrade-package pandas
uv sync --locked
```

Gunakan `uv add` untuk mencatat dependensi proyek. Instalasi melalui pip saja tidak memperbarui deklarasi dan lockfile. Hindari cell instalasi paket yang ikut dijalankan saat Run All.

## Pemeriksaan development

Dari root repository:

```bash
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
go build ./...
uv lock --check
uv run --locked python -c "import pandas, matplotlib, yaml, ipykernel, jupyterlab"
```

Tes Go memakai data kecil, filesystem sementara, dan respons HTTP simulasi. Menjalankan tes tidak mengunduh OSM atau menjalankan training penelitian.

## Git dan artefak lokal

Commit kode, notebook, `README.md`, `config.yaml`, `go.mod`, `go.sum`, `pyproject.toml`, `uv.lock`, dan `.python-version` sesuai perubahan yang dikerjakan.

`.venv/`, cache bytecode Python, checkpoint notebook, `bin/`, serta direktori `cache/` dan `output/` di dalam `workdirs/` diabaikan Git. `.venv` dapat dibuat ulang melalui `uv sync --locked`. Snapshot OSM dan CSV yang diperlukan untuk reproduksi penelitian perlu dicadangkan terpisah karena tidak ikut commit.

Perbarui README bersama perubahan command, konfigurasi, dependensi, atau alur kerja agar contoh penggunaan tetap sesuai implementasi.

## Pemecahan masalah

| Gejala | Pemeriksaan |
|---|---|
| `uv` tidak ditemukan | Periksa instalasi dan PATH dari terminal sistem |
| `ModuleNotFoundError` | Jalankan `uv sync --locked`, lalu periksa kernel dan `sys.executable` |
| `uv sync --locked` menolak lockfile | Periksa perubahan `pyproject.toml`; jalankan `uv lock` bila perubahan dependensi memang disengaja |
| Kernel proyek belum tampil | Daftarkan kernel dengan perintah `ipykernel install --sys-prefix`, lalu muat ulang JupyterLab |
| Snapshot/CSV tidak ditemukan | Periksa `--workdir`, `output_dir`, dan urutan `download` lalu `analyze` |
| Hasil notebook masih lama | Jalankan ulang pembacaan CSV atau restart kernel dan Run All |
| Overpass menolak atau membatasi request | Periksa log; client melakukan retry untuk HTTP 429/502/503/504. Hindari pengulangan `--force` tanpa kebutuhan |

## Referensi

- [Proyek Python acuan](https://github.com/davidimannuel/uwika-ta-31123022-2026).
- [Pengelolaan proyek uv](https://docs.astral.sh/uv/guides/projects/).
- [Integrasi uv dan Jupyter](https://docs.astral.sh/uv/guides/integration/jupyter/).
- [Model graf jaringan OSMnx](https://osmnx.readthedocs.io/en/stable/getting-started.html#modeling-a-network).
- [Perhitungan jarak OSMnx/Haversine](https://github.com/gboeing/osmnx/blob/v2.1.1/osmnx/distance.py#L68-L112).

Filter walking dan model graf pada repository ini mengacu pada OSMnx, dengan implementasi Go yang dikerjakan per komponen. Kesamaan query saja belum menjamin kesamaan graf akhir atau hasil training dengan proyek Python.

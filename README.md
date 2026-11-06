# Go Running Route Q-Learning

Penulisan ulang core TA **Sistem Rekomendasi Rute Lari Menggunakan Algoritma Q-Learning Berbasis Data OpenStreetMap di Daerah Dukuh Kupang, Surabaya** ke Go. Target penelitian adalah rute loop 5 km dengan toleransi 10%.

Go menangani pengambilan data, pembentukan graf, training Q-learning, dan ekspor peta. Python membaca CSV keluaran Go untuk analisis, tabel, dan diagram melalui JupyterLab. Setiap percobaan dikelompokkan dalam satu **workdir** yang berisi konfigurasi, cache, dan hasil.

## Status implementasi

| Komponen | Status |
|---|---|
| CLI `download` | Pengambilan OSM melalui Overpass, cache, dan penyimpanan JSON |
| CLI `analyze` | Membentuk graf mentah, mengekspor CSV berskor, dan membuat `road-analysis.html` |
| CLI `serve` | Server lokal dan index otomatis untuk file HTML |
| Graf | Edge terarah dua arah, edge paralel, asal way, tag asli, dan panjang Haversine |
| Python | Environment analisis dengan uv, pandas, matplotlib, PyYAML, dan JupyterLab |
| Notebook analisis | `analisa.ipynb` untuk graf; `training.ipynb` untuk evaluasi, kurva training, diagnosis, overlap, dan perbandingan profil/state |
| Kandidat action | Dibentuk sekali berdasarkan akses dan panjang positif; lookup per node tanpa alokasi |
| CLI `train` | State A/B/C, environment, reward, Q-table, seed independen, history CSV, dan evaluasi greedy |
| Comfort score | Pemetaan highway/surface/width mengikuti core Python acuan; digunakan bersama oleh CSV dan peta |
| Peta interaktif | `road-analysis.html` untuk graf; `training/routes.html` untuk seluruh seed evaluasi, marker start/finish, dan warna per kilometer |

Graf saat ini belum melalui simplifikasi, pemilihan komponen terhubung, atau pemotongan batas wilayah seperti alur akhir OSMnx. Setiap pasangan node berurutan pada way menghasilkan edge maju dan balik. CSV mencatat **edge terarah**, sehingga jumlah edge bukan jumlah ruas fisik unik.

## Struktur repository

```text
cmd/cli/             Entry point, command download/analyze/train/serve, dan penulisan hasil
internal/config/     Pembacaan dan validasi konfigurasi YAML
internal/logger/     Structured logging berbasis log/slog
internal/htmlserver/ Server khusus HTML dan template index otomatis
internal/osm/        Overpass, cache, graf, jarak, dan parsing atribut OSM
internal/qlearning/ State, action mask, reward, environment, Q-table, training, dan metrik
internal/analysis/  Ekspor CSV, penyimpanan Q-table, dan template peta HTML
analysis/            Modul Python untuk membaca CSV, memeriksa data, dan membuat laporan
workdirs/            Konfigurasi dan artefak masing-masing percobaan
playground/          Percobaan lokal, diabaikan Git
analisa.ipynb        Parameter dan pemanggilan laporan analisis graf
training.ipynb       Parameter dan pemanggilan laporan hasil training
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

## Command penting: empat workdir utama

Jalankan dari root repository. Pilih command sesuai percobaan yang ingin dijalankan.

| Workdir | Skenario | Training per command | Seed |
|---|---|---|---|
| `workdirs/scenario-a` | A: node sekarang | Empat profil 75k/150k, epsilon dinamis/horizon 50k | Lima tetap: 0–4 |
| `workdirs/scenario-b` | B: node + progress bin | Empat profil yang sama | Lima tetap: 0–4 |
| `workdirs/scenario-c` | C: node + sebelumnya + progress bin | Empat profil yang sama | Lima tetap: 0–4 |
| `workdirs/testing` | B, titik `-7.281518, 112.710178` | 150k episode per seed, horizon 50k | Lima acak per command |

### Snapshot dan analisis bersama A/B/C

Unduh sekali, lalu gunakan file yang sama untuk ketiga skenario. Analisis graf A/B/C cukup dilakukan pada workdir sumber ini karena inputnya sama:

```bash
go run ./cmd/cli download --workdir workdirs/dukuh-kupang
go run ./cmd/cli analyze --workdir workdirs/dukuh-kupang
```

### Training A, B, atau C

Masing-masing command berikut menjalankan empat profil dari awal, dengan lima seed per profil. Hasil disimpan di workdir skenario masing-masing; `--snapshot` menunjuk input bersama:

```bash
# Scenario A
go run ./cmd/cli train --workdir workdirs/scenario-a \
  --snapshot workdirs/dukuh-kupang/output/osm.json

# Scenario B
go run ./cmd/cli train --workdir workdirs/scenario-b \
  --snapshot workdirs/dukuh-kupang/output/osm.json

# Scenario C
go run ./cmd/cli train --workdir workdirs/scenario-c \
  --snapshot workdirs/dukuh-kupang/output/osm.json
```

Untuk satu profil saja, tambahkan `--profile` (juga berlaku pada scenario-a/c):

```bash
go run ./cmd/cli train --workdir workdirs/scenario-b \
  --snapshot workdirs/dukuh-kupang/output/osm.json \
  --profile horizon-50000-total-150000
```

### Download, analyze, dan training testing

Testing memakai snapshot sendiri di sekitar koordinat config. Lima run × 150k berarti total 750.000 episode; epsilon tetap 0,05 setelah episode ke-50.000:

```bash
go run ./cmd/cli download --workdir workdirs/testing
go run ./cmd/cli analyze --workdir workdirs/testing
go run ./cmd/cli train --workdir workdirs/testing
```

Seed otomatis dicetak dan disimpan di `workdirs/testing/output/training/metadata.json`. Untuk mengulang, ganti `runs: 5` dengan `seeds: [...]` dari hasil tersebut. Training berikutnya menimpa hasil training dalam workdir/profil yang dipilih.

### Buka peta, notebook, dan bantuan

```bash
go run ./cmd/cli serve --dir workdirs --port 8080
```

Buka [index peta lokal](http://127.0.0.1:8080). Peta training A/B/C berada di `<scenario>/output/training/<profile>/routes.html`; testing di `testing/output/training/routes.html`. Peta analisis berada di `<workdir>/output/road-analysis.html`.

```bash
uv run --locked jupyter lab
```

Buka `analisa.ipynb` atau `training.ipynb`, lalu ubah `WORKDIR` pada cell awal. Gunakan `dukuh-kupang` untuk analisis graf bersama, `scenario-a/b/c` untuk metrik masing-masing skenario, atau `testing` untuk keduanya. Pilih **Restart Kernel and Run All** setelah mengganti data/parameter.

```bash
go run ./cmd/cli --help
go run ./cmd/cli train --help
# Optional diagnostic logging:
go run ./cmd/cli analyze --workdir workdirs/testing --debug
# Refresh the OSM snapshot when needed:
go run ./cmd/cli download --workdir workdirs/testing --force
```

`analyze` memerlukan `osm.json` dan `train` tidak mengunduh data otomatis. Pada clone baru, jalankan `download` dan `analyze` dahulu: output/cache memang tidak ikut commit.

## Konfigurasi dan workdir

Workdir adalah folder yang langsung memuat `config.yaml`. Contoh yang tersedia menggunakan nama `dukuh-kupang`; nama workdir bebas dan dapat diganti sesuai percobaan.

```text
workdirs/dukuh-kupang/
  config.yaml
  cache/                 Dibuat saat download
  output/
    osm.json             Snapshot hasil download
    nodes.csv            Tabel node hasil analyze
    edges.csv            Tabel edge dan skor hasil analyze
    road-analysis.html   Peta interaktif seluruh graf
    training-run.json    Status percobaan training terbaru dan seed aktual
    training/            Hasil training lengkap terakhir (dibuat oleh train)
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

Parameter `map` wajib diisi. Path direktori harus relatif dan tetap berada di dalam workdir. Field YAML yang tidak dikenal ditolak. Konfigurasi tidak memerlukan opsi ekspor: `analyze` selalu menulis kedua CSV dan satu HTML peta.

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

### Membentuk graf dan mengekspor hasil analisis

```bash
go run ./cmd/cli analyze --workdir workdirs/dukuh-kupang
```

Command membaca `output_dir/osm.json` tanpa menghubungi Overpass, membentuk graf mentah, lalu selalu menulis atau menimpa `nodes.csv`, `edges.csv`, dan `road-analysis.html` di direktori yang sama. Snapshot sumber tetap dipertahankan. Jika JSON belum tersedia, jalankan `download` terlebih dahulu.

| File | Kolom |
|---|---|
| `nodes.csv` | `node_id`, `latitude`, `longitude` |
| `edges.csv` | `from`, `to`, `key`, `way_id`, `length_m`, `name`, `highway`, `surface`, `width`, `access`, `highway_score`, `surface_score`, `width_score`, `comfort_score` |

Node diurutkan berdasarkan ID dan edge berdasarkan `(from, to, key)`. `way_id` menghubungkan segmen ke way asal. Key dimulai dari `0` untuk setiap pasangan terarah `(from, to)` dan bertambah hanya pada edge paralel dengan pasangan yang sama. Contoh: `1→2` dan `1→3` masing-masing mendapat key `0`; edge kedua `1→2` mendapat key `1`. Edge arah balik dan paralel tetap menjadi baris terpisah.

`length_m` dihitung per segmen dengan Haversine menggunakan radius bumi 6.371.009 meter dan disimpan tanpa pembulatan. Tag `width` diekspor sebagai teks asli, sehingga kosong, `3`, `3 m`, dan `3;4` tetap dapat dibedakan. Empat atribut analisis adalah **highway, surface, width, dan access**.

Setiap file ditulis melalui file sementara sebelum mengganti hasil lama. Penggantian berlaku per file, bukan satu transaksi untuk seluruh CSV dan HTML. Gunakan hasil setelah command selesai dengan sukses.

### Training dan evaluasi greedy

Tersedia **tiga workdir penelitian** dan satu workdir testing. Workdir penelitian masing-masing memakai satu `config.yaml`: `workdirs/scenario-a/`, `workdirs/scenario-b/`, dan `workdirs/scenario-c/`. Setiap config memuat empat `training.profiles`; parameter bersama cukup ditulis sekali:

```yaml
qlearning:
  scenario: B
  training:
    alpha: 0.15
    gamma: 0.95
    epsilon_start: 1.0
    epsilon_end: 0.05
    seeds: [0, 1, 2, 3, 4]
    max_workers: 3
    profiles:
      - name: dynamic-75000
        episodes: 75000
        epsilon_decay_episodes: null
      - name: dynamic-150000
        episodes: 150000
        epsilon_decay_episodes: null
      - name: horizon-50000-total-75000
        episodes: 75000
        epsilon_decay_episodes: 50000
      - name: horizon-50000-total-150000
        episodes: 150000
        epsilon_decay_episodes: 50000
```

Bagian `map` dan `environment` lengkap tersedia dalam ketiga config tersebut. Profil yang dijalankan:

| Profil | Episode per seed | Horizon epsilon |
|---|---:|---|
| `dynamic-75000` | 75.000 | Mengikuti total episode |
| `dynamic-150000` | 150.000 | Mengikuti total episode |
| `horizon-50000-total-75000` | 75.000 | 50.000 |
| `horizon-50000-total-150000` | 150.000 | 50.000 |

Semuanya memakai seed `[0, 1, 2, 3, 4]`, alpha `0.15`, gamma `0.95`, epsilon `1.0` sampai `0.05`, dan maksimal tiga worker. Hasil 150k dilatih dari awal; tidak melanjutkan Q-table 75k. A hanya mengamati node sekarang; B menambahkan progress bin; C menambahkan node sebelumnya. Node sebelumnya saat reset adalah `-1`, sedangkan node/bin `0` tetap sah.

Gunakan satu snapshot lokal yang sama untuk membandingkan konfigurasi. Contoh dari root repository:

```bash
go run ./cmd/cli download --workdir workdirs/dukuh-kupang
go run ./cmd/cli analyze --workdir workdirs/dukuh-kupang
go run ./cmd/cli train \
  --workdir workdirs/scenario-b \
  --snapshot workdirs/dukuh-kupang/output/osm.json
go run ./cmd/cli serve --dir workdirs --port 8080
```

`train` tidak mengunduh data. Tanpa `--snapshot`, inputnya adalah `<workdir>/<output_dir>/osm.json`. Path `--snapshot` relatif terhadap direktori terminal saat command dijalankan. Secara default, `train` menjalankan semua profil secara berurutan, memakai satu graf yang disiapkan sekali. Seed otomatis juga dipilih sekali dan dipakai bersama pada semua profil agar perbandingan tetap memakai seed yang sama. Lima seed di dalam setiap profil dapat berjalan paralel sesuai `max_workers`; setiap training tetap dimulai dengan Q-table kosong.

Untuk menjalankan atau mengulang satu profil saja:

```bash
go run ./cmd/cli train --workdir workdirs/scenario-b \
  --snapshot workdirs/dukuh-kupang/output/osm.json \
  --profile horizon-50000-total-75000
```

Pilih `scenario-b/output/training/horizon-50000-total-75000/routes.html` dari index server. Training penelitian tidak dijalankan otomatis oleh setup atau tes.

### Workdir testing dengan seed acak

`workdirs/testing/config.yaml` disiapkan untuk titik start `(-7.281518, 112.710178)`, radius bbox 3.025 meter, dan skenario B. Training memakai 150.000 episode per seed, `runs: 5`, maksimal tiga worker, serta horizon epsilon 50.000 episode. Epsilon turun dari `1.0` ke `0.05`, lalu tetap `0.05` untuk 100.000 episode sisanya. Totalnya 750.000 episode per command `train`.

Config ini tidak memakai `seeds` tetap atau daftar profil. Setiap command `train` memilih lima seed otomatis yang berbeda dalam batch tersebut; seed dicetak dan disimpan di `output/training/metadata.json` serta marker run agar percobaan bisa diulang. Analisis graf tidak memakai seed.

Dari root repository, unduh snapshot untuk koordinat config, analisis graf, lalu jalankan training:

```bash
go run ./cmd/cli download --workdir workdirs/testing
go run ./cmd/cli analyze --workdir workdirs/testing
go run ./cmd/cli train --workdir workdirs/testing
go run ./cmd/cli serve --dir workdirs --port 8080
```

`download` memakai cache bila tersedia; tambahkan `--force` jika membutuhkan snapshot baru dari server. Peta analisis berada di `testing/output/road-analysis.html`; peta hasil training berada di `testing/output/training/routes.html`. Untuk notebook `analisa.ipynb` dan `training.ipynb`, ubah `WORKDIR` menjadi `ROOT / "workdirs" / "testing"`, lalu restart kernel dan jalankan semua cell. Folder `output/` dan `cache/` tetap diabaikan Git.

Untuk mengubah titik awal, edit bagian `map`, lalu jalankan `download` dan `analyze` sebelum training. Untuk mengulang seed tertentu, ganti `runs: 5` dengan `seeds: [...]` dari metadata hasil sebelumnya. Training baru yang berhasil mengganti hasil training lama dalam workdir ini; snapshot dan hasil analisis tetap tersedia.

### Parameter training dan artefak

Untuk percobaan pendek, buat workdir sendiri dengan bagian `map` yang sama. Config tanpa `profiles` tetap mendukung satu training sederhana:

```yaml
qlearning:
  scenario: B
  environment:
    target_distance_m: 5000
    distance_tolerance: 0.10
    progress_bins: 16
    max_steps: 220
    allow_revisit: true
    revisit_after_ratio: 0.50
    max_node_visits: 2
    max_snap_distance_m: 100
  training:
    episodes: 100
    epsilon_decay_episodes: null
    alpha: 0.15
    gamma: 0.95
    epsilon_start: 1.0
    epsilon_end: 0.05
    runs: 2
    max_workers: 2
```

`profiles` dapat berisi satu atau beberapa entri. Setiap nama harus unik, maksimal 64 karakter, memakai huruf kecil/angka/tanda hubung, dan setiap profil wajib menentukan jumlah episode positif. Nama menjadi nama folder hasil. Jika memakai `profiles`, letakkan `episodes` dan `epsilon_decay_episodes` di setiap profil; parameter durasi datar hanya dipakai ketika `profiles` tidak ada.

Environment dan parameter training yang tidak ditulis memakai nilai default di atas, kecuali default `episodes=10000` dan `max_workers=1`. `scenario` wajib untuk `train`. Reward tidak menjadi field YAML: konstanta berada di `internal/qlearning/reward.go`.

- `runs: 2` menghasilkan dua seed otomatis yang berbeda. Seed dicetak dan disimpan sebelum training; tanpa `runs`/`seeds`, dipilih satu seed otomatis.
- Untuk reproduksi, ganti `runs` dengan `seeds: [0, 1, 2, 3, 4]` atau angka hasil sebelumnya. `runs` dan `seeds` tidak boleh bersamaan; daftar kosong/duplikat ditolak. Nilai `0` adalah seed sah.
- `epsilon_decay_episodes: null` mengikuti jumlah episode. Nilai positif menentukan horizon tetap; `0` ditolak. Epsilon dihitung satu kali per episode, dan evaluasi final selalu memakai `epsilon=0`.
- RNG adalah `math/rand/v2` PCG dengan pasangan `(seed, 0)`, dipertahankan antar-episode. Evaluasi memakai `(seed+100000, 0)`. Seed maksimum adalah `18446744073709451615` agar offset tidak overflow. Seed Go tidak menghasilkan urutan NumPy yang sama.
- Setiap worker memiliki environment/Q-table/RNG sendiri; graf dan persiapan skor dibagi read-only. Induk menulis history melalui channel terbatas. Hasil diurutkan kembali berdasarkan urutan seed konfigurasi, sehingga penjadwalan worker tidak mengubah artefak numerik.
- Start adalah node terdekat dari `map.latitude/longitude`; jarak snap di atas batas atau start tanpa kandidat action menyebabkan error. Pemilihan start tidak diam-diam berpindah ke node lain.

Artefak setiap profil berada di `<workdir>/<output_dir>/training/<nama-profil>/`. Config sederhana tanpa `profiles` memakai `<workdir>/<output_dir>/training/`:

| Berkas | Isi |
|---|---|
| `history.csv` | Seluruh episode semua seed: reward, jarak, keberhasilan, comfort, overlap, return progress, terminasi, Q-state, epsilon, dan TD-error |
| `evaluation.csv` | Satu evaluasi greedy final untuk setiap seed, termasuk hasil gagal |
| `seed-<seed>/qtable.csv` | Q-table lengkap dan state kosong untuk dimuat kembali melalui `analysis.ReadQTable` |
| `seed-<seed>/evaluation-steps.csv` | State/action tiap langkah greedy, koordinat, atribut, komponen reward, terminasi, dan nilai Q yang dibaca |
| `routes.html` | Pilihan semua seed, rute representatif default, warna kilometer, marker start/finish, dan popup atribut/reward |
| `metadata.json` | Config, seed aktual, hash snapshot, RNG, aturan reward/mask/state, dan metode jarak/overlap |

History ditulis bertahap, bukan ditahan seluruhnya di RAM. `NaN` pada rasio tanpa observasi berarti data tidak tersedia; pandas membacanya sebagai nilai kosong. Overlap memakai `(jumlah lintasan - jumlah ruas fisik unik) / jumlah lintasan`, tanpa bobot panjang. Pada graf mentah, geometri adalah garis endpoint: arah balik dan edge paralel bergeometri identik dihitung sebagai ruas fisik yang sama. Identitas action tetap `(next_node, key)`.

`TrainStep` mengembalikan perhitungan reward dan Bellman yang sama dengan training untuk unit test/manual check. CSV langkah di atas merupakan **evaluasi greedy**, sehingga `q_updated=false`; kolom target/TD kosong. Config trace episode training belum ditambahkan. Grafik laporan dikerjakan di Python dari CSV melalui `training.ipynb`; `analisa.ipynb` tetap khusus analisis graf.

Peta memilih loop valid dengan galat jarak terkecil, lalu comfort/return progress lebih tinggi dan seed lebih kecil. Jika semua gagal, galat jarak tetap menjadi urutan pertama, lalu jarak akhir ke start. Rute gagal tetap diberi label gagal. Marker kilometer diinterpolasi pada segmen untuk visualisasi, bukan rekaman GPS. Internet diperlukan untuk Leaflet dan tile OSM; buka melalui `serve` agar browser mengirim referer yang sesuai.

Hasil baru disiapkan di folder sementara sebelum mengganti **folder profil yang sedang dijalankan**. Hasil profil lain tetap tersimpan, termasuk ketika memakai `--profile`. Seed lama di dalam profil tersebut dibuang setelah run baru berhasil. Jika gagal/dibatalkan, hasil lengkap sebelumnya dipertahankan. `output_dir/training-run.json` mencatat status seluruh profil yang dipilih pada command terakhir; `training/<nama-profil>-run.json` mencatat status percobaan terakhir untuk profil tersebut. Periksa status sebelum memakai hasil. Command `download`/`analyze` dan snapshot tidak ditimpa oleh training.

**Batas kesetaraan dengan Python:** reward, state, mask, jadwal epsilon, konfigurasi, dan rumus Q mengikuti acuan. Graf Go masih berupa segmen mentah tanpa simplifikasi/seleksi komponen/truncation lanjutan. Jarak ke start memakai Haversine sesuai kesepakatan, sedangkan acuan memakai Euclidean UTM. Access di Go menerima satu nilai saja. Karena jumlah langkah/struktur graf dan RNG berbeda, jangan mengharapkan rute atau angka hasil Python identik. Batas 220 langkah tetap dipertahankan; pada graf mentah, ini dapat menghentikan rute sebelum mencapai 5 km. Keberhasilan unit test bukan validasi lapangan.

### Comfort score dan peta analisis

Scoring berada di `internal/qlearning/scoring.go`, mengikuti aturan aktif pada `rute_lari_core.py` proyek acuan. `qlearning.ScoreEdge(edge)` menghitung skor dari atribut edge yang sudah diparse. CSV dan HTML pada `internal/analysis` memakai fungsi yang sama; JavaScript hanya memilih warna berdasarkan nilai yang sudah dihitung.

```text
comfort_score = 0.55 × highway_score + 0.30 × surface_score + 0.15 × width_score
```

| Komponen | Aturan |
|---|---|
| Highway | footway/pedestrian/living_street/residential/tertiary: +1; kategori lain: −1; kosong: −0,5 |
| Surface | asphalt/concrete/paving_stones/paved: +1; kategori lain: −1; kosong: −0,5 |
| Width | Satu lebar positif ≥3 m: +1; positif <3 m: −1; kosong/invalid/gabungan: −0,5 |

Gabungan highway/surface menggunakan skor minimum. Width diparse sekali per way ketika `LoadOSM` membentuk edge, lalu disimpan sebagai `OSMEdge.Width` (meter) dan `WidthValid`. Nilai kosong/invalid/gabungan disimpan sebagai `0` dengan `WidthValid=false`; scoring memakai kedua field tersebut tanpa parsing ulang. Tag asli tetap berada di `Tags["width"]`. Parser `ParseWidthM` terpusat di `internal/osm/width.go`. Width menerima angka tunggal, koma desimal, serta satuan m/meter/metre/meters/metres; tidak merata-ratakan nilai gabungan. Tag asli tetap tersedia di CSV dan popup. Nilai gabungan disimpan tanpa clipping. Access tidak masuk comfort score; kandidat action sudah difilter berdasarkan akses, sedangkan mask dinamis dan penalti perjalanan dihitung oleh environment Q-learning. Skor adalah preferensi penelitian, bukan hasil survei kenyamanan atau pengukuran keamanan.

Setelah `analyze` selesai, jalankan server dari root proyek:

```bash
go run ./cmd/cli serve
```

Buka index yang dicetak CLI, lalu pilih `workdirs/dukuh-kupang/output/road-analysis.html`. Dengan port default, URL-nya adalah [peta analisis](http://127.0.0.1:8080/files/workdirs/dukuh-kupang/output/road-analysis.html). Jika `output_dir` berbeda, pilih path yang sesuai di index.

- Pilihan warna **Netral, Highway (default), Surface, Access, Comfort score** memakai satu dataset dan legenda dinamis.
- Arahkan pointer ke ruas untuk tooltip; klik untuk tag asli, panjang, identitas edge, tiga skor komponen, dan skor gabungan.
- Garis raw dengan pasangan endpoint yang sama digambar sekali. Popup menyediakan pilihan seluruh edge maju/balik dan paralel yang diwakili; identitas `(from,to,key)` dan data graf tetap utuh. Ini pengelompokan tampilan, bukan simplifikasi graf atau hitungan ruas fisik unik.
- Jika kategori edge yang bertumpuk berbeda pada tema aktif, garis diberi warna **Different overlapping categories**; pilih edge pada popup untuk memeriksa masing-masing nilai.
- Comfort memakai interval acuan: <−0,5; −0,5 hingga <0; netral (toleransi numerik 1e−12); >0 hingga 0,5; dan >0,5. Warna tidak mengubah skor.
- **Show nodes** awalnya mati. Pada zoom ≥17, node dalam viewport ditampilkan, maksimum 2.000; jika batas tercapai, zoom lebih dekat. Tombol **Show entire graph** mengembalikan cakupan seluruh node.
- Marker **START (input)** memakai pin hijau dengan simbol ▶ putih mengikuti proyek acuan. Klik pin untuk popup berisi judul START dan koordinat latitude/longitude dalam dua kolom. Ujung pin menunjukkan koordinat config, bukan node START hasil snapping. Basemap dapat memuat edit OSM yang lebih baru daripada snapshot graf.

HTML menyematkan seluruh data dan tidak membaca CSV/JSON melalui browser. Internet masih diperlukan untuk Leaflet dan tile OSM. Ulangi `analyze` setelah snapshot berubah, lalu reload halaman peta. Ekspor peta tidak menjalankan training atau melakukan unduhan OSM.

### Membuka peta HTML melalui server lokal

```bash
go run ./cmd/cli serve
```

Default server adalah `http://127.0.0.1:8080/index.html`. CLI mencetak URL aktif ke stdout dan log ke stderr. Buka URL tersebut di browser; index menampilkan file `.html` secara rekursif dari direktori saat command dijalankan. File di `playground/` dan hasil dalam `workdirs/` ikut ditemukan meskipun diabaikan Git. Nama berekstensi `.HTML` juga diterima.

Untuk membatasi pencarian atau mengganti port:

```bash
go run ./cmd/cli serve --dir playground --port 8081
go run ./cmd/cli serve --dir workdirs/dukuh-kupang/output
```

`--dir` adalah direktori yang disajikan, bukan pembacaan konfigurasi workdir; default `.`. Command ini tidak memerlukan `config.yaml` atau snapshot OSM. `--port 0` memilih port kosong secara otomatis dan mencetak URL yang dipilih. Tekan **Ctrl+C** untuk menghentikan server. Tidak ada server yang dibuka ke jaringan luar; alamat selalu `127.0.0.1`.

Index dibuat di memori pada `/` dan `/index.html`, tanpa menulis file ke disk. Refresh index untuk melihat HTML yang baru dibuat. Link berkas memakai `/files/<path-relatif>`, misalnya `/files/playground/map.html` pada direktori default. HTML asli bernama `index.html` tetap dapat dibuka melalui `/files/index.html`.

Whitelist berlaku pada daftar dan akses HTTP: hanya file HTML reguler di dalam direktori terpilih yang disajikan. File CSV, YAML, JSON, gambar, JavaScript, CSS, direktori tersembunyi, dan symlink tidak disajikan. Peta harus menyematkan data dan kode lokal ke dalam HTML, atau memakai aset eksternal seperti Leaflet melalui HTTPS. HTML yang meminta file JSON/CSV lokal terpisah akan mendapat 404.

Contoh Leaflet memerlukan internet untuk library dan tile peta OpenStreetMap. Membukanya melalui HTTP lokal menyediakan origin halaman untuk request tile; server ini tidak mengunduh atau mem-proxy tile dan tidak menghubungi Overpass. Koneksi internet, header browser, dan kebijakan penyedia tetap memengaruhi akses tile. Lihat [kebijakan tile OpenStreetMap](https://operations.osmfoundation.org/policies/tiles/).

Tambahkan `--debug` untuk mencatat method dan path request HTTP. Log startup/shutdown tetap tampil pada level INFO.

### Log, bantuan, dan binary

Command `train` menampilkan ringkasan teks melalui stdout: konfigurasi profil, satu baris ketika setiap seed selesai, lalu jumlah loop greedy, rata-rata galat jarak seluruh seed, durasi, dan lokasi hasil. Tidak ada log episode atau JSON rutin pada tampilan default. Urutan baris seed mengikuti waktu selesai; urutan CSV tetap mengikuti daftar seed.

Tambahkan `--debug` pada `train` untuk detail progres episode dan log diagnostik JSON melalui stderr. Command lain tetap menggunakan log level `INFO` ke atas. Contoh:

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

## Kandidat action Q-learning

`internal/qlearning/action.go` menyediakan tabel kandidat edge dari graf yang sudah dimuat. `NewActions(graph)` melakukan filter akses dan panjang sekali di awal. `Available(nodeID)` langsung membaca tabel tersebut tanpa mengulang loop, parsing, atau alokasi hasil:

```go
actions := qlearning.NewActions(graph)
candidates := actions.Available(nodeID)
for _, edge := range candidates {
    fmt.Println(edge.To, edge.Key, edge.LengthM)
}
```

`access` kosong, `yes`, dan `permissive` diizinkan; nilai lain ditolak. Spasi dan kapitalisasi dinormalisasi. Nilai diperiksa sebagai satu string utuh; gabungan titik koma maupun representasi list Python/JSON ditolak sebagai nilai yang tidak dikenal. `foot=yes` tidak mengesampingkan aturan akses penelitian. Panjang harus positif dan finite. Edge paralel tetap terpisah berdasarkan `(to, key)`; pengurutan kandidat tidak mengubah adjacency graf.

Tabel menyimpan `map[int][]*osm.OSMEdge`: ada tambahan memori untuk map dan pointer kandidat, sementara atribut edge dan map tag tetap menggunakan data graf yang sama. Slice dari `Available` maupun `Graph.OutgoingEdges`, edge, dan map `Tags` bersifat read-only; jangan dimodifikasi, di-append, atau diurutkan langsung. Environment akan memiliki storage sendiri untuk mask perjalanan. Bangun ulang `Actions` jika graf dimuat ulang atau kebijakan akses berubah. Tidak diperlukan buffer hasil pada setiap pemanggilan `Available`.

Komponen ini hanya menyaring akses dan panjang. `Environment.Available()` menerapkan mask revisit/closing pada kandidat tersebut. CLI `train` melakukan snapping start dengan Haversine, lalu memakai kandidat yang sama untuk eksplorasi, greedy, dan bootstrap. Kandidat diurutkan secara numerik berdasarkan `(next_node, key)`: tujuan lebih dahulu, lalu key. Key mengikuti urutan `0, 1, 2, ..., 10, 11`; slice adjacency graf tetap tidak diubah. Urutan ini berbeda dari pembandingan key sebagai teks pada acuan. Metadata run baru mencatat urutan numerik; metadata/hasil lama dipertahankan sesuai aturan ketika hasil tersebut dibuat.

Jalankan tes dan benchmark lokal:

```bash
go test ./internal/osm ./internal/qlearning
go test ./internal/qlearning -run '^$' -bench . -benchmem
```

## Pembagian tanggung jawab package

- `osm` menyimpan data dan graf: `LoadOSM`, `OutgoingEdges`, `Edges`, dan `ParseWidthM`.
- `qlearning` menentukan aturan penelitian: skor, state, action mask, reward, transisi, Bellman update, training, dan metrik.
- `analysis` menghasilkan artefak melalui `NodeToCSV(writer, graph)`, `EdgeToCSV(writer, graph)`, dan `MapToHTML(writer, graph, latitude, longitude)`.
- CLI mengoordinasikan pembacaan konfigurasi, pembentukan graf, logging, dan penulisan file hasil.

Dependency berjalan dari `analysis` ke `qlearning` dan `osm`; `qlearning` memakai `osm`. Package `osm` tidak mengimpor aturan penelitian maupun ekspor analisis. CSV/peta memakai seluruh graf, sedangkan tabel kandidat action memakai filter akses dan panjang. `Graph.Edges()` menyediakan salinan nilai edge untuk pengurutan ekspor tanpa mengubah urutan adjacency graf; map tag tetap dibagi sebagai data read-only.

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

Implementasi Python dipisahkan dari notebook:

| Modul | Tanggung jawab |
|---|---|
| `analysis/graph.py` | `GraphAnalysis`: membaca CSV graf, memeriksa konsistensi, serta membuat tabel dan diagram atribut |
| `analysis/training.py` | `TrainingAnalysis`: membaca hasil training Go, memeriksa metadata/history, dan membuat laporan metrik |
| `analysis/common.py` | Resolusi path workdir, pembacaan `output_dir`, dan pemeriksaan environment notebook |

Notebook hanya berisi penjelasan, parameter, pemanggilan bagian laporan, dan hasil tampilannya. Parameter graf adalah `WORKDIR`, `TOP_N`, `SAMPLE_ROWS`, dan `LENGTH_BINS`. Parameter training adalah `WORKDIR`, `PROFILES`, `ROLLING_WINDOW`, `TAIL_EPISODES`, `PLOT_EVERY`, dan `CHUNK_SIZE`; daftar pembanding serta alamat server peta juga tetap ada pada cell awal. Ubah `WORKDIR` ke `ROOT / "workdirs" / "testing"` untuk percobaan titik baru.

Cell setup memuat ulang modul dengan `importlib.reload`. Setelah mengubah modul atau parameter, jalankan ulang dari cell setup (**Restart Kernel and Run All** untuk seluruh laporan). Data graf tersedia melalui `report.nodes` dan `report.edges`; evaluasi training melalui `report.results`, serta ringkasan profil melalui `report.profile_summary` setelah bagian perbandingan dijalankan. Jangan mengubah reward atau aturan Q-learning di modul Python ini; implementasinya tetap di Go.

Modul juga dapat dipakai dari file Python pada root repository:

```python
from analysis.training import TrainingAnalysis

report = TrainingAnalysis("workdirs/testing", rolling_window=50)
report.load()
report.show_configurations()
report.show_profiles()
report.compare_profiles()
```

Contoh tersebut membaca training yang sudah selesai. Pengambilan OSM, ekspor CSV graf, dan training tetap dijalankan melalui CLI Go.


### Analisis setelah training

```bash
uv run --locked jupyter lab training.ipynb
```

Buka `training.ipynb`, pilih `WORKDIR` pada cell pertama, lalu **Restart Kernel and Run All**. Default adalah `workdirs/scenario-b`, yang sudah memiliki hasil tersimpan. `PROFILES=None` memuat seluruh profil selesai; gunakan misalnya `PROFILES=["dynamic-75000"]` untuk satu profil. Notebook tidak menjalankan training atau mengubah Q-table/hasil mentah.

Notebook mengikuti tampilan pascatraining acuan:

- Tabel evaluasi greedy vertikal (metrik sebagai baris, seed sebagai kolom), konfigurasi aktual dari metadata, dan agregat seluruh seed.
- Enam panel training (3 × 2), dengan ukuran, warna, judul, label, dan legenda mengikuti fungsi grafik pada proyek acuan: keberhasilan loop, reward, jarak, comfort, Q-state, dan epsilon.
- Diagnosis: jarak akhir ke start, progres pulang, kesempatan closing, TD-error, serta empat alasan terminasi.
- Satu panel overlap untuk seluruh episode, tanpa pemisahan berhasil/gagal; basisnya jumlah lintasan ruas fisik.
- Perbandingan jumlah episode dipisahkan menurut epsilon dinamis/horizon tetap, masing-masing berupa empat grafik garis (2 × 2) seperti section 3 notebook acuan; overlap evaluasi ditampilkan terpisah per strategi. Ringkasan 10.000 episode terakhir tetap diekspor ke CSV.
- Perbandingan A/B/C hanya ketika hasil tersedia dengan durasi, seed, snapshot, start, reward, dan parameter bersama yang sepadan. Data yang belum ada tidak diganti nol; status kesepadanan dicatat pada tabel.
- Tautan ke peta HTML Go melalui server `serve`; peta tidak dimuat otomatis saat analisis tabel/grafik.

History dibaca per 50.000 baris untuk membatasi pemakaian RAM. Kurva utama merata-ratakan seed per episode sebelum rata-rata bergerak 50 episode; khusus overlap memakai jumlah/cacah observasi agar nilai kosong tidak mengubah bobot. Default `PLOT_EVERY=1` menggambar seluruh episode seperti acuan. Bila diubah untuk pratinjau ringan, pengurangan titik dilakukan sesudah smoothing; ringkasan numerik tetap memakai seluruh data. Sumbu comfort mengikuti acuan 0–1 bila mencakup data, dan diperluas jika ada skor negatif/di luar rentang agar tidak tersembunyi.

Metadata dan marker status diperiksa agar percobaan yang gagal/masih berjalan tidak diam-diam ditampilkan memakai hasil lama. Seed dibaca sebagai teks untuk mempertahankan presisi `uint64`. Config YAML menentukan lokasi data, sedangkan parameter eksperimen dibaca dari metadata tersimpan.

Tabel turunan (`configurations.csv`, `profile_summary.csv`, `last_window_summary.csv`, dan `state_comparison.csv`) disimpan di `output_dir/analysis/training/`. Subfolder tiap profil memuat grafik `training`, `diagnosis`, dan `overlap` dalam PNG/SVG, rata-rata per episode, overlap bergerak, statistik jendela akhir, serta tabel terminasi training/evaluasi. Ringkasan diagnosis per profil tersedia pada `diagnosis_summary.csv`; grafik perbandingan durasi dan tabelnya ada di `comparison/dynamic/` atau `comparison/horizon-<episode>/`, sedangkan `evaluation_overlap` menggabungkan panel strategi epsilon. File hasil training asli tetap dipertahankan. Angka rata-rata evaluasi mencakup rute berhasil dan gagal; comfort bukan persentase keamanan/kenyamanan.

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

Tes Go memakai data kecil, filesystem sementara, dan respons HTTP simulasi. Scoring diperiksa dengan hitungan manual serta 112 kasus beku dari fungsi Python acuan; hash sumber tercatat di `internal/qlearning/testdata/scoring_reference.json`. Ekspor HTML diuji untuk identitas edge paralel, skor, escaping tag, ID besar, overwrite, dan error penulisan. Tes training menambahkan hitungan Q independen, fase arah 2.490 m, batas 4.500/5.500 m, penalti buntu sekali, paralel key, reproducibility seed, kesetaraan worker berurutan/paralel, dan evaluasi ulang dari Q-table CSV. Menjalankan tes tidak mengunduh OSM atau menjalankan training penelitian.

### Memeriksa graf secara manual

Contoh [playground/graph_test.go](playground/graph_test.go) membaca snapshot pada workdir `dukuh-kupang`, membentuk graf dengan `LoadOSM`, lalu memanggil `Graph.PrintInfo(nodeID)`:

```bash
go test -v ./playground -count=1
```

Lokasi snapshot mengikuti `output_dir` pada `workdirs/dukuh-kupang/config.yaml`. Default node yang diperiksa adalah `9181658878`; ubah `nodeID` di fungsi `TestGraphPrintInfo` untuk memilih node lain dari `nodes.csv`. Tidak ada data sintetis atau unduhan jaringan pada contoh ini. Jika snapshot belum ada, pemeriksaan dilewati. Kolom output dijelaskan sebelum daftar edge. `-count=1` menjalankan ulang agar hasil pemeriksaan tidak berasal dari cache tes.

## Git dan artefak lokal

Commit kode, notebook (`analisa.ipynb` dan `training.ipynb`), `README.md`, `config.yaml`, `go.mod`, `go.sum`, `pyproject.toml`, `uv.lock`, dan `.python-version` sesuai perubahan yang dikerjakan.

`.venv/`, cache bytecode Python, checkpoint notebook, `bin/`, `playground/`, serta direktori `cache/` dan `output/` di dalam `workdirs/` diabaikan Git. `.venv` dapat dibuat ulang melalui `uv sync --locked`. Snapshot OSM dan CSV yang diperlukan untuk reproduksi penelitian perlu dicadangkan terpisah karena tidak ikut commit.

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
| Port server sudah dipakai | Pilih `serve --port 8081` atau `--port 0` |
| HTML tidak muncul di index | Periksa `--dir`, ekstensi `.html`, dan apakah path tersembunyi atau symlink; refresh index |
| Peta menampilkan tile diblokir | Buka melalui URL HTTP dari `serve`; periksa Console/Network browser dan kebijakan penyedia tile |
| Overpass menolak atau membatasi request | Periksa log; client melakukan retry untuk HTTP 429/502/503/504. Hindari pengulangan `--force` tanpa kebutuhan |

## Referensi

- [Proyek Python acuan](https://github.com/davidimannuel/uwika-ta-31123022-2026).
- [Pengelolaan proyek uv](https://docs.astral.sh/uv/guides/projects/).
- [Integrasi uv dan Jupyter](https://docs.astral.sh/uv/guides/integration/jupyter/).
- [Model graf jaringan OSMnx](https://osmnx.readthedocs.io/en/stable/getting-started.html#modeling-a-network).
- [Perhitungan jarak OSMnx/Haversine](https://github.com/gboeing/osmnx/blob/v2.1.1/osmnx/distance.py#L68-L112).

Filter walking dan model graf pada repository ini mengacu pada OSMnx, dengan implementasi Go yang dikerjakan per komponen. Kesamaan query saja belum menjamin kesamaan graf akhir atau hasil training dengan proyek Python.

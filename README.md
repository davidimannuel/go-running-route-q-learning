# Running Route Q-Learning

Core Go untuk TA rekomendasi rute lari loop berbasis OpenStreetMap. Go menangani
OSM, graf, reward, Q-learning, CSV, dan peta HTML. Python/Jupyter membaca hasil
untuk analisis dan diagram laporan.

Target rute adalah kembali ke node start dengan panjang **4.500–5.500 meter**.
Skor highway/surface/width adalah preferensi penelitian, bukan ukuran keamanan
atau hasil survei kenyamanan pelari.

## Struktur proyek

```text
cmd/cli/             download, analyze, train, serve, dan penyimpanan hasil
internal/osm/        Overpass, cache, graf mentah, Haversine, parsing width
internal/qlearning/  state A/B/C, action, reward, environment, Q-table, metrik
internal/analysis/   CSV, HTML peta, dan halaman metadata hasil
internal/config/     konfigurasi YAML dan validasi
internal/htmlserver/ navigasi hasil HTML lokal dan halaman metadata arsip
analysis/            modul Python untuk analisis CSV dan plotting
workdirs/training/   analisis dan 12 profil penelitian A/B/C
workdirs/testing/    12 profil A/B/C dengan lima seed acak untuk uji parameter
analisa.ipynb        pilihan parameter dan laporan analisis graf
training.ipynb       pilihan profil, laporan training, dan perbandingan state
playground/          pemeriksaan manual lokal; diabaikan Git
```

Graf utama memakai segmen OSM mentah tanpa simplifikasi. Setiap pasangan node
berurutan pada way menghasilkan edge maju dan balik. Atribut asli dipertahankan
per segmen. Seleksi komponen dan truncation setara OSMnx belum diterapkan.

## Setup

Gunakan Go **1.27.2+**, uv, dan Python **3.13.x**. Dari root repository:

```bash
go mod download
uv python install 3.13
uv sync --locked
uv run --locked python -m ipykernel install --sys-prefix \
  --name go-running-route-analysis \
  --display-name "Running Route Analysis (Python 3.13)"
```

`uv sync --locked` membuat `.venv` dan memasang versi dependensi dari `uv.lock`.
Tidak perlu mengaktifkan environment secara manual. Pilih kernel
**Running Route Analysis (Python 3.13)** di editor notebook.

## Command utama

Workdir penelitian menggunakan koordinat `-7.282187, 112.709282`. Workdir testing
menggunakan `-7.281518, 112.710178`. Lokasi dan radius dapat diubah di config.
Semua command berikut dijalankan dari root repository. Jika `osm.json` sudah
tersedia, lanjutkan dengan `analyze` atau `train` sesuai kebutuhan.

```bash
# Snapshot dan analisis penelitian.
go run ./cmd/cli download --workdir workdirs/training
go run ./cmd/cli analyze --workdir workdirs/training

# Semua 12 profil penelitian (A/B/C, masing-masing lima seed).
go run ./cmd/cli train --workdir workdirs/training

# Atau pilih satu dari empat konfigurasi; ganti awalan b menjadi a/c bila perlu.
go run ./cmd/cli train --workdir workdirs/training \
  --profile b-dynamic-150000
go run ./cmd/cli train --workdir workdirs/training \
  --profile b-dynamic-300000
go run ./cmd/cli train --workdir workdirs/training \
  --profile b-horizon-100000-total-150000
go run ./cmd/cli train --workdir workdirs/training \
  --profile b-horizon-100000-total-300000

# Snapshot dan analisis testing.
go run ./cmd/cli download --workdir workdirs/testing
go run ./cmd/cli analyze --workdir workdirs/testing

# Semua 12 profil testing dengan lima seed acak bersama.
go run ./cmd/cli train --workdir workdirs/testing

# Atau pilih satu konfigurasi testing; awalan a/b/c menentukan scenario.
go run ./cmd/cli train --workdir workdirs/testing --profile b-dynamic-150000
go run ./cmd/cli train --workdir workdirs/testing --profile b-dynamic-300000
go run ./cmd/cli train --workdir workdirs/testing --profile b-horizon-100000-total-150000
go run ./cmd/cli train --workdir workdirs/testing --profile b-horizon-100000-total-300000

# Navigasi peta dan informasi training.
go run ./cmd/cli serve --dir workdirs --port 8080

# Notebook, bantuan, atau binary lokal.
uv run --locked jupyter lab
go run ./cmd/cli --help
go build -o bin/route-cli ./cmd/cli
```

Buka **http://127.0.0.1:8080/**. Pilih peta dari daftar, gunakan pencarian atau
filter jenis hasil, lalu pilih **Recorded settings** untuk melihat konfigurasi
hasil tersebut. Peta ditampilkan di panel sebelah daftar sehingga navigasi tetap
tersedia. **Open separately** membuka hasil pada tab lain.

`train` tanpa `--profile` menjalankan seluruh profil secara berurutan.
Kedua workdir memakai 12 profil × lima seed, total **13.500.000 episode per
workdir**. Training memakai seed tetap 0–4; testing memilih lima seed acak sekali
per command untuk seluruh profil yang dipilih. Setiap profil dimulai dari Q-table
kosong. Hasil yang sudah ada tidak membuat profil dilewati otomatis.

Profil yang tersedia untuk `--profile`:

| Konfigurasi | Scenario A | Scenario B | Scenario C |
|---|---|---|---|
| Dinamis 150k | `a-dynamic-150000` | `b-dynamic-150000` | `c-dynamic-150000` |
| Dinamis 300k | `a-dynamic-300000` | `b-dynamic-300000` | `c-dynamic-300000` |
| Horizon 100k, total 150k | `a-horizon-100000-total-150000` | `b-horizon-100000-total-150000` | `c-horizon-100000-total-150000` |
| Horizon 100k, total 300k | `a-horizon-100000-total-300000` | `b-horizon-100000-total-300000` | `c-horizon-100000-total-300000` |

Keempat konfigurasi berlaku untuk `workdirs/training` dan `workdirs/testing`,
dengan nama profil yang sama. Scenario dipilih melalui profil di YAML. Tidak ada flag
`--scenario`. Jalankan `go run ./cmd/cli --help` untuk ringkasan command dan contoh
terbaru; `download --help`, `analyze --help`, `train --help`, dan `serve --help`
menampilkan panduan yang sama.

Tambahkan `--debug` untuk log rinci. Tampilan training default hanya parameter,
satu baris per seed selesai, dan summary. Log command lain memakai level info.

## Config dan profil

Setiap workdir mempunyai satu `config.yaml`; tidak ada subfolder input.
Konfigurasi lengkap tersedia di [training](workdirs/training/config.yaml) dan
[testing](workdirs/testing/config.yaml).

```yaml
schema_version: 1
map:
  latitude: -7.282187
  longitude: 112.709282
  radius_m: 3025
  timeout_sec: 90
output_dir: output
cache_dir: cache
qlearning:
  environment:
    target_distance_m: 5000
    distance_tolerance: 0.10
    progress_bins: 25
    max_steps: 500
    allow_revisit: true
    revisit_after_ratio: 0.50
    max_node_visits: 2
    max_snap_distance_m: 100
  reward:
    road:
      highway: {weight: 0.55, priority: 1.0, other: -1.0, missing: -0.5}
      surface: {weight: 0.30, priority: 1.0, other: -1.0, missing: -0.5}
      width: {weight: 0.15, priority: 1.0, other: -1.0, missing: -0.5}
      width_threshold_m: 3.0
    return_phase_ratio: 0.50
    outward_penalty_scale: 0.60
    return_reward_scale: 1.20
    permissive_penalty_per_100m: -0.10
    revisit_penalty: -5.0
    success_bonus: 100.0
    over_distance_penalty: -50.0
    no_action_penalty: -50.0
    step_limit_penalty: -50.0
  training:
    alpha: 0.15
    gamma: 0.95
    epsilon_start: 1.0
    epsilon_end: 0.05
    seeds: [0, 1, 2, 3, 4]
    max_workers: 5
    profiles:
      - name: a-dynamic-150000
        scenario: A
        episodes: 150000
        epsilon_decay_episodes: null
      - name: b-dynamic-150000
        scenario: B
        episodes: 150000
        epsilon_decay_episodes: null
```

Setiap profil menentukan `name`, `scenario`, `episodes`, dan
`epsilon_decay_episodes`. Nama unik memakai huruf kecil, angka, dan hyphen;
nama tersebut juga menjadi direktori hasil. Config penelitian memuat empat
kombinasi per skenario: dinamis 150k/300k dan horizon 100k dengan total 150k/300k.
Config testing memakai 12 profil yang sama; koordinat dan cara pemilihan seed
tetap berbeda dari workdir penelitian.

`epsilon_decay_episodes: null` menurunkan epsilon secara linear dari 1,00 ke 0,05
sepanjang total episode profil. Horizon tetap 100k mencapai 0,05 pada episode
100.000, lalu bertahan sampai training selesai. Evaluasi akhir selalu greedy,
`epsilon=0`. Semua profil penelitian memakai 25 bin, graf, reward, dan lima seed
yang sama; setiap profil mulai dari Q-table kosong. Rancangan ini membandingkan
state A/B/C, jadwal epsilon pada durasi yang sama, dan durasi pada jadwal yang sama.

Untuk seed acak, hapus `seeds` dan gunakan `runs: 5`. Seed dipilih **sekali per
command** lalu dipakai bersama oleh seluruh profil. Salin `resolved_seeds` dari
metadata ke `seeds` untuk mengulangi batch. Jangan mengisi `runs` dan `seeds`
bersamaan. Memilih satu profil pada command terpisah dengan seed otomatis akan
menghasilkan daftar seed baru.

Reward yang tidak dicantumkan mendapat default dari `DefaultRewardConfig()`.
Override parsial didukung, termasuk angka nol. Bobot digunakan langsung tanpa
normalisasi, dan skor tidak di-clip. Perubahan reward berlaku pada training,
evaluasi, serta skor CSV/peta yang dibuat oleh `analyze` berikutnya. Jalankan
ulang `analyze` setelah mengubah parameter scoring agar peta analisis diperbarui.
Kategori prioritas highway/surface dan kebijakan akses tetap berada di kode.

Config lama dengan `qlearning.scenario` dan tanpa profil masih dapat dibaca.
Profil lama tanpa `scenario` mewarisi nilai itu; config aktif memakai scenario
pada setiap profil. Field YAML yang tidak dikenal ditolak.

Rancangan final memakai outward 0,60, return 1,20, revisit −5, 25 bin, dan
batas 500 langkah; alpha 0,15 dan gamma 0,95. Kedua workdir memakai reward yang
sama. Pada 7 November 2026, seluruh output eksperimen lama beserta arsip dan
baseline dihapus atas permintaan pengguna. Snapshot OSM yang sama disiapkan
kembali dari cache lokal. Pengguna menyelesaikan seluruh rancangan final pada
7 November 2026: 12 profil, 60 training, dan 13.500.000 episode; greedy valid
31/60 (A 1/20, B 18/20, C 12/20). Jumlah loop per lima seed:

| Konfigurasi | A | B | C |
|---|---:|---:|---:|
| Dinamis 150k | 0 | 5 | 0 |
| Dinamis 300k | 1 | 5 | 3 |
| Horizon 100k, total 150k | 0 | 3 | 4 |
| Horizon 100k, total 300k | 0 | 5 | 5 |

Seluruh history diperiksa dan 60 evaluasi direproduksi dari Q-table tersimpan.
Ringkasan audit ada di `workdirs/training/output/analysis/final-experiment-audit.json`.
Hasil berlaku pada graf/start/seed yang diuji; bukan jaminan optimalitas atau
validasi lapangan. Pertahankan konfigurasi dan hasil ini untuk analisis BAB IV.

Snapshot final memiliki **26.345 node dan 58.020 edge terarah**, dengan start node
`9620836306`. SHA-256 snapshot:
`f0a5885db46c2f9cd962674aaeaa82f7d5c69579d5ba5d80641f727b6e20f964`.
CSV, Q-table, peta hasil, serta cache merupakan artefak lokal yang tidak masuk
Git. Unduhan OSM baru dapat mengubah graf dan hasil, meskipun seed sama.

## Output dan metadata

```text
output/
  osm.json                       snapshot OSM
  download.json                  config, hash snapshot, timestamp, status cache
  nodes.csv / edges.csv          tabel graf dan skor atribut
  road-analysis.html             peta graf seluruh area
  analysis.json / analysis.html  config dan provenance analisis
  training-run.json              status batch terakhir dan seed aktual
  training/
    b-dynamic-150000-run.json     status percobaan terakhir profil ini
    b-dynamic-150000/
      metadata.json              config efektif, seed, graf, reward, evaluasi
      report.html                informasi training yang dapat dibaca di browser
      routes.html                peta evaluasi seluruh seed
      history.csv                metrik tiap episode dan seed
      evaluation.csv             satu evaluasi greedy per seed
      seed-0/
        metadata.json            seed, config efektif, dan evaluasi seed ini
        qtable.csv
        evaluation-steps.csv     transisi, komponen reward, dan nilai Q
  analysis/                      tabel/diagram turunan dari Python
```

Metadata training menyimpan parameter efektif setelah profil diterapkan,
konfigurasi workdir, waktu mulai/selesai UTC, seed aktual, hash SHA-256 snapshot,
jumlah node/edge, start dan jarak snap, reward, aturan algoritma, serta informasi
build yang tersedia. **Editing YAML tidak mengubah metadata hasil lama.**
Halaman report menampilkan konfigurasi yang direkam, bukan membaca config saat ini.
Peta `routes.html` menyediakan layer **Comfort score** dan **Access** untuk graf
sekitar rute. Keduanya tersembunyi saat dibuka (**Roads hidden**); pilih tema di
kontrol layer dan gunakan **Selected route** untuk menampilkan/menyembunyikan rute.
Warna dan kategori mengikuti peta analisis; popup tetap memuat setiap edge terarah.

Di panel rute, **Route distance** memilih **All kilometers** atau satu rentang
kilometer. Bagian lain tetap samar untuk konteks; **Fit selected route**
menyesuaikan peta dengan pilihan tersebut. Batas kilometer diinterpolasi pada
garis tampilan, termasuk sisa jarak setelah KM 5, tanpa mengubah edge atau metrik.
**Highlight repeated roads** menambahkan garis tepi putus-putus pada ruas yang
dilalui ulang; **Direction arrows** menambahkan petunjuk arah. Keduanya awalnya
mati. Penandaan pengulangan selalu memakai seluruh rute seed yang dipilih,
meskipun tampilan sedang dibatasi ke satu kilometer.

Klik ruas yang dilalui ulang untuk memilih **Route traversal** pada popup.
Setiap lintasan menampilkan step, rentang jarak kumulatif, arah/action, atribut,
dan komponen reward asli. Identitas ruas mengikuti metrik overlap: koordinat
endpoint yang sama digabung tanpa membedakan arah/key. Pilihan seed, metrik,
marker input/start/finish/KM, serta layer Comfort score/Access tetap tersedia.
Peta hasil final yang sudah ada telah diperbarui; tidak perlu training ulang.

Nilai metrik yang tidak terdefinisi disimpan sebagai NaN pada CSV dan null pada JSON.

Training menyiapkan satu direktori sementara lalu memublikasikannya setelah
seluruh artefak selesai. Pembatalan/kegagalan mempertahankan hasil lengkap
sebelumnya; file `*-run.json` merekam percobaan terbaru. Menjalankan ulang satu
profil hanya menimpa profil itu. File download/analisis ditulis secara atomik
per file; gunakan hasil setelah command sukses.

## OSM, cache, dan graf

`download` memakai Overpass bbox dengan filter walking. `radius_m` adalah jarak
pusat ke sisi bbox dalam meter. Cache memakai hash endpoint dan query; cache
rusak diambil ulang, tanpa masa kedaluwarsa otomatis. Untuk memperbarui:

```bash
go run ./cmd/cli download --workdir workdirs/training --force
```

`analyze` dan `train` memakai snapshot lokal tanpa download. Untuk memakai
snapshot lain secara eksplisit:

```bash
go run ./cmd/cli train --workdir workdirs/testing \
  --snapshot workdirs/training/output/osm.json --profile b-dynamic-150000
```

Snapshot harus sesuai titik start; CLI menolak snap melebihi
`max_snap_distance_m` atau start tanpa action legal. Jangan mengunduh ulang saat
membandingkan A/B/C jika ingin mempertahankan graf yang sama.

Graf adalah directed multigraph. Action memakai `(next_node, key)`; key dimulai
0 untuk setiap pasangan terarah `(from,to)`. Kandidat diurutkan numerik menurut
tujuan dan key. Setiap edge tetap terpisah pada CSV, termasuk arah balik dan
edge paralel. Panjang memakai Haversine, radius bumi 6.371.009 m.

Atribut reward adalah highway, surface, width. Highway prioritas: footway,
pedestrian, living_street, residential, tertiary. Surface prioritas: asphalt,
concrete, paving_stones, paved. Width diparse sekali ketika load: angka tunggal,
koma desimal, dan satuan meter didukung; gabungan/invalid mendapat skor missing.
Tag asli tetap tersedia pada CSV/popup. Access scalar kosong/yes/permissive
lolos; `foot=yes` tidak override dan nilai gabungan tidak di-split.

State A memakai current node; B menambahkan progress bin; C menambahkan previous
node. Mask akses, revisit, dan closing dipakai konsisten pada eksplorasi, greedy,
dan bootstrap. Overlap memakai hitungan lintasan ruas fisik tanpa membedakan arah,
sehingga segmen pendek/panjang memiliki bobot hitungan sama.

## Analisis Python

Jalankan `analisa.ipynb` dan `training.ipynb` dengan **Restart Kernel and Run All**.
Default keduanya `workdirs/training`; ganti `WORKDIR` menjadi `testing` sesuai
kebutuhan. `PROFILES=None` memuat semua profil selesai; contohnya
`PROFILES=["b-dynamic-150000"]` untuk satu profil. Profil belum tersedia dilaporkan
pada tabel ketersediaan. Jika belum ada hasil selesai, jalankan Go terlebih dahulu.

Notebook hanya menyimpan pilihan data, pemanggilan, dan hasil. Implementasi
analisis berada di `analysis/graph.py` dan `analysis/training.py`; Python tidak
menjalankan training atau memperbarui Q-table. Analisis training memisahkan
perbandingan durasi berdasarkan scenario dan mencocokkan A/B/C berdasarkan
jumlah episode serta horizon epsilon. Graf, seed, reward, dan parameter lain
harus cocok sebelum grafik perbandingan state dibuat.

Riwayat dibaca per chunk (`CHUNK_SIZE=50000`). `ROLLING_WINDOW=50`,
`TAIL_EPISODES=10000`, dan `PLOT_EVERY=1` mengikuti tampilan acuan.
`PLOT_EVERY` mengurangi titik sesudah rolling dihitung; tidak mengubah data sumber.
Grafik mencakup reward, jarak, keberhasilan, TD-error, diagnosis, dan overlap
seluruh episode. Judul grafik cukup menampilkan scenario dan nama profil,
misalnya `Scenario A | a-dynamic-150000`.
Hasil turunan disimpan di `output/analysis/`.

Untuk dependensi baru gunakan `uv add <package>` atau `uv add --group dev <package>`,
lalu commit `pyproject.toml` dan `uv.lock` bersama. Setup ulang memakai
`uv sync --locked`. Jangan mengedit `uv.lock` secara manual.

## Development dan batas interpretasi

```bash
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
go build ./...
go test ./playground -run TestGraphPrintInfo -v -count=1
```

Tes menggunakan fixture lokal dan hitungan reward/Q independen. Pemeriksaan
manual playground membaca snapshot `workdirs/training`; ubah nodeID untuk
memeriksa node lain. `cache/`, `output/`, `.venv`, dan playground diabaikan Git.
Server hanya menyajikan HTML; file CSV, JSON, YAML, hidden path, dan symlink tidak
dapat diunduh langsung. Endpoint detail merender metadata hasil yang diizinkan
menjadi HTML. Tile OSM memerlukan internet dan server lokal dengan referrer normal.

Graf mentah, jarak Haversine, urutan key numerik, dan RNG PCG berbeda dari
OSMnx/proyeksi UTM/NumPy. Seed yang sama tidak menjamin rute identik lintas bahasa.
Workdir aktif memakai batas 500 langkah dan lima worker untuk percobaan graf
mentah. Unit test bukan validasi lapangan; rute gagal tetap dilaporkan. Jangan
menyimpulkan kualitas dari reward atau satu loop berhasil saja.

## Referensi

- [Proyek Python acuan](https://github.com/davidimannuel/uwika-ta-31123022-2026).
- [Pengelolaan proyek uv](https://docs.astral.sh/uv/guides/projects/).
- [Integrasi uv dan Jupyter](https://docs.astral.sh/uv/guides/integration/jupyter/).
- [Model graf jaringan OSMnx](https://osmnx.readthedocs.io/en/stable/getting-started.html#modeling-a-network).
- [Perhitungan jarak OSMnx/Haversine](https://github.com/gboeing/osmnx/blob/v2.1.1/osmnx/distance.py#L68-L112).

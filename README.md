# Running Route Q-Learning

Core Go untuk TA rekomendasi rute lari loop berbasis OpenStreetMap. Go menangani
OSM, graf, reward, Q-learning, CSV, dan peta HTML. Python/Jupyter membaca hasil
untuk analisis dan diagram laporan.

**Katalog publik:** [Buka katalog rute lari](https://davidimannuel.github.io/go-running-route-q-learning/).
GitHub Pages sudah aktif; katalog menampilkan hasil pilihan yang disimpan
sebelumnya, tanpa menjalankan training di browser.

Target rute adalah kembali ke node start mendekati **5.000 meter**. Konfigurasi
aktif memakai toleransi **20% (4.000–6.000 meter)**; checkpoint lama memakai 10%.
Skor highway/surface/width adalah preferensi penelitian, bukan ukuran keamanan
atau hasil survei kenyamanan pelari.

## Struktur proyek

```text
cmd/cli/             run, download, analyze, train, catalog, export, serve
internal/osm/        Overpass, cache, graf mentah, Haversine, parsing width
internal/qlearning/  state A/B/C, action, reward, environment, Q-table, metrik
internal/analysis/   CSV, HTML peta, dan halaman metadata hasil
internal/config/     konfigurasi YAML dan validasi
internal/htmlserver/ navigasi hasil HTML lokal dan halaman metadata arsip
analysis/            modul Python untuk analisis CSV dan plotting
workdirs/training/   analisis dan 12 profil penelitian A/B/C
workdirs/testing/    profil aktif pilihan dengan lima seed acak untuk uji parameter
analisa.ipynb        pilihan parameter dan laporan analisis graf
training.ipynb       pilihan profil, laporan training, dan perbandingan state
web-catalog/         situs katalog statis untuk GitHub Pages
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

Koordinat dan radius aktif mengikuti `map` di config masing-masing workdir.
Penelitian memakai acuan `-7.281518, 112.710178`; testing dapat memakai lokasi
pilihanmu. Setelah mengubah lokasi/radius, gunakan `run` agar snapshot sesuai.
Semua command berikut dijalankan dari root repository. Jika `osm.json` sudah
tersedia, lanjutkan dengan `analyze` atau `train` sesuai kebutuhan.

```bash
# Download → analyze → train dalam satu command (semua profil aktif).
go run ./cmd/cli run --workdir workdirs/testing
go run ./cmd/cli run --workdir workdirs/training

# Atau hanya satu profil training, setelah download dan analyze.
go run ./cmd/cli run --workdir workdirs/testing \
  --profile b-horizon-100000-total-300000

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

# Profil testing aktif dengan lima seed acak bersama.
go run ./cmd/cli train --workdir workdirs/testing

# Testing kini hanya mengaktifkan profil B horizon berikut.
go run ./cmd/cli train --workdir workdirs/testing --profile b-horizon-100000-total-300000

# Buat ulang HTML dari hasil tersimpan, tanpa training ulang.
go run ./cmd/cli export --workdir workdirs/training
go run ./cmd/cli export --workdir workdirs/testing

# Navigasi peta dan informasi training.
go run ./cmd/cli serve --dir workdirs --port 8080

# Notebook, bantuan, atau binary lokal.
uv run --locked jupyter lab
go run ./cmd/cli --help
go build -o bin/route-cli ./cmd/cli
```

Buka **http://127.0.0.1:8080/**. Pilih peta dari daftar, gunakan pencarian atau
filter jenis hasil, lalu pilih **Recorded settings** untuk melihat konfigurasi
hasil tersebut. Pencarian `training/` atau `testing/` membatasi workdir;
contoh `training/ b-horizon` juga menyaring profil. Nama workdir yang cocok
tidak dicocokkan ke folder internal `output/training/` atau teks tombol. Peta ditampilkan di panel sebelah daftar sehingga navigasi tetap
tersedia. **Open separately** membuka hasil pada tab lain. Pemilihan hasil
dan **Refresh results** memuat ulang iframe dengan URL baru; server mengirim
HTML tanpa cache. Setelah mengubah kode Go/template server, hentikan `serve`
dengan Ctrl+C lalu jalankan kembali agar server memakai versi kode terbaru.

`run` selalu menjalankan download → analyze → train secara berurutan dan berhenti
pada tahap pertama yang gagal atau dibatalkan. Setelah mengubah `map.latitude`,
`map.longitude`, atau `map.radius_m`, gunakan command ini agar snapshot disiapkan
untuk query terbaru. Cache dipilih berdasarkan query; tambahkan `--force` bila
perlu mengambil data baru dari Overpass walaupun cache tersedia. `--debug` juga
didukung. Config dan nama profil diperiksa sebelum download; tidak ada opsi
`--snapshot` pada `run`. Jangan mengedit config selama command berjalan.

Hasil tahap yang sudah selesai tetap tersimpan jika tahap berikutnya gagal;
ini bukan transaksi untuk seluruh output. Hasil training profil lain yang tidak
dipilih tetap dipertahankan sesuai perilaku `train`; periksa metadata saat
membandingkan hasil dari lokasi berbeda. Gunakan `output_dir` berbeda jika ingin
menyimpan hasil percobaan sebelumnya. `run` tidak menjalankan notebook, export,
atau menambahkan rute ke katalog.

`train` tanpa `--profile` menjalankan seluruh profil secara berurutan.
Rancangan penuh memakai 12 profil × lima seed, total **13.500.000 episode**.
Profil aktif mengikuti YAML; testing saat ini memilih B horizon100k-total300k,
yaitu **1.500.000 episode** untuk lima seed. Training memakai seed tetap
0–4; testing memilih lima seed acak sekali
per command untuk seluruh profil yang dipilih. Setiap profil dimulai dari Q-table
kosong. Hasil yang sudah ada tidak membuat profil dilewati otomatis.

Profil yang tersedia untuk `--profile`:

| Konfigurasi | Scenario A | Scenario B | Scenario C |
|---|---|---|---|
| Dinamis 150k | `a-dynamic-150000` | `b-dynamic-150000` | `c-dynamic-150000` |
| Dinamis 300k | `a-dynamic-300000` | `b-dynamic-300000` | `c-dynamic-300000` |
| Horizon 100k, total 150k | `a-horizon-100000-total-150000` | `b-horizon-100000-total-150000` | `c-horizon-100000-total-150000` |
| Horizon 100k, total 300k | `a-horizon-100000-total-300000` | `b-horizon-100000-total-300000` | `c-horizon-100000-total-300000` |

Keempat konfigurasi ada di `workdirs/training`; testing saat ini mengaktifkan
hanya `b-horizon-100000-total-300000`. Scenario dipilih melalui profil di YAML. Tidak ada flag
`--scenario`. Jalankan `go run ./cmd/cli --help` untuk ringkasan command dan contoh
terbaru; `run --help`, `download --help`, `analyze --help`, `train --help`, dan `serve --help`
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
  latitude: -7.281518
  longitude: 112.710178
  radius_m: 3025
  timeout_sec: 90
output_dir: output
cache_dir: cache
qlearning:
  environment:
    target_distance_m: 5000
    distance_tolerance: 0.20
    progress_bins: 25
    max_steps: 500
    allow_revisit: true
    revisit_after_ratio: 0.50
    max_node_visits: 2
    max_snap_distance_m: 100
  reward:
    road:
      highway: {weight: 0.55, priority: 1.0, secondary_priority: 0.5, other: -10.0, missing: -2.0}
      surface: {weight: 0.30, priority: 1.0, other: -1.0, missing: -0.5}
      width: {weight: 0.15, priority: 1.0, other: -1.0, missing: -0.5}
      width_threshold_m: 3.0
    return_phase_ratio: 0.50
    outward_penalty_scale: 0.0
    return_reward_scale: 1.20
    permissive_penalty_per_100m: -0.10
    revisit_penalty: -10.0
    distance_error_penalty_per_100m: -5.0
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
Config testing dapat memilih subset dari definisi profil yang sama; saat ini
hanya B horizon100k-total300k aktif. Koordinat dan cara pemilihan
seed berbeda dari workdir penelitian.

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

`highway.secondary_priority` hanya berlaku untuk tag `highway=secondary`.
Kedua workdir menetapkan `0.5`: prioritas utama tetap `+1`; highway
service/kategori lain saat ini `−10`, dan highway kosong `−2`. Surface/width
memakai missing `−0.5`. `secondary_link` tetap memakai `other`. Dengan
surface/width kosong, skor gabungan secondary adalah `0.05`, dibanding service
`−5.725`. Aturan ini preferensi penelitian lokal, bukan penilaian keamanan.
Jika parameter dihilangkan atau null, secondary memakai `other` seperti versi
lama; angka `0` merupakan override yang sah. Default core tetap kompatibel dengan
aturan lama. Nilai eksplisit ikut direkam pada config/road_score metadata dan
halaman Recorded settings. CSV, peta analisis, peta rute, dan reward training
memakai pemetaan yang sama; hasil tersimpan tidak berubah sebelum command terkait
dijalankan ulang.

### Penalti selisih jarak akhir

Keputusan pengguna 9 November: `distance_error_penalty_per_100m: -5.0` pada
training/testing. Komponen tambahan dihitung **sekali saat episode selesai**:

```text
reward_distance_error = -5 * abs(total_distance_m - target_distance_m) / 100
```

Jarak memakai total panjang rute, bukan jarak langsung ke start. Selisih 50m
mendapat −2,5; 250m mendapat −12,5; 500m mendapat −25; 1.000m mendapat −50.
Kekurangan dan kelebihan diperlakukan sama. Pada langkah nonterminal komponen
ini nol. Nilai nol/field yang dihilangkan menonaktifkannya, termasuk ketika
membaca metadata lama. Nilai harus finite dan tidak positif.

Penalti ditambahkan pada sukses, over_distance, step_limit, maupun buntu, termasuk
buntu sebelum action pertama. Buntu setelah bergerak memasukkannya sekali ke
StopPenalty bersama penalti buntu, sebelum pembaruan Q; reward terminal tidak
bootstrap. Success bonus +100 dan penalti gagal yang sudah ada tetap berlaku.
Contoh loop 5.500m: bonus sukses +100 dan penalti galat −25, sebelum reward ruas,
akses, arah, dan revisit lain dijumlahkan.

Terminasi over_distance, mask closing, toleransi 20%, state/bin, epsilon, graf,
seed dan reward lain tidak berubah. Eksperimen ini **bukan** mode eksplorasi
melewati batas maksimum. Trace baru menambahkan `reward_distance_error`, popup
rute menampilkan komponennya, dan metadata mencatat rumus/aplikasi serta scheme
`configurable_signed_unclipped_distance_error_v2` bila koefisien tidak nol.

Training dengan penalti baru selesai22.18 WIB, menghasilkan22/60 loop valid
(rincian checkpoint di bawah). Untuk menyimpan pembanding,
pilih `output_dir` lain sebelum training berikutnya dan gunakan snapshot lama
melalui `--snapshot workdirs/training/output/osm.json`; editing YAML tidak
mengubah hasil tersimpan. Command train tetap mengganti profil pada output tujuan.

Config lama dengan `qlearning.scenario` dan tanpa profil masih dapat dibaca.
Profil lama tanpa `scenario` mewarisi nilai itu; config aktif memakai scenario
pada setiap profil. Field YAML yang tidak dikenal ditolak.

Checkpoint 7 November memakai outward 0,60, return 1,20, revisit −5, 25 bin, dan
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
validasi lapangan. Hasil 7 November memakai filter akses way saja. Mulai
9 November, action juga memeriksa akses node asal/tujuan. Angka di atas adalah
checkpoint 7 November; output terbaru dapat sudah dilatih ulang dengan versi
kebijakan berbeda. Periksa CSV/metadata setiap profil sebelum membandingkan.
Audit/replay di atas dilakukan sebelum perubahan mask. Pisahkan output baru agar
baseline tidak tertimpa; command `train` tetap mengganti hasil di output tujuan.

**Pembanding workdir training — 9 November 2026, selesai 14.31 WIB (`highway.other=-1`).**
Seluruh12 profil/lima seed sudah memakai `edge_and_endpoint_whitelist_v1`:
akses way serta kedua endpoint. Config reward, snapshot, start, dan seed sama
antar A/B/C. Evaluasi akhir greedy menghasilkan **9/60 loop valid**:

| Konfigurasi | A | B | C |
|---|---:|---:|---:|
| Dinamis 150k | 0/5 | 2/5 | 1/5 |
| Dinamis 300k | 0/5 | 0/5 | 0/5 |
| Horizon 100k, total 150k | 0/5 | 1/5 | 1/5 |
| Horizon 100k, total 300k | 0/5 | 2/5 | 2/5 |

Ada44 over_distance dan7 no_available_action; tidak ada step_limit (maksimum285
langkah). Seluruh kegagalan tidak pernah memiliki closing action legal menurut
metrik tersimpan. Sembilan loop merupakan delapan urutan edge unik; panjang
4.526,24–5.307,47m. Overlap rata-rata rute valid B5,33% (n=5), C4,09% (n=4);
ini hanya subset berhasil, bukan seluruh evaluasi. Pada10.000 episode terakhir,
keberhasilan training horizon-total300k adalah B26,45%/C18,37%; evaluasi tetap
2/5 pada keduanya. Training bereksplorasi dengan epsilon0,05, evaluasi memakai0.

Pemeriksaan membaca13.500.000 baris history dan mencocokkan60 evaluasi dengan
trace untuk langkah, panjang, reward, comfort, serta overlap; tidak ada lintasan
ke node/way yang ditolak whitelist snapshot. Ini belum replay seluruh Q-table
atau validasi lapangan. Angka31/60 di atas dan audit/diagram dalam `analysis/`
bertanggal7 November adalah riwayat sebelum mask node, bukan hasil batch ini.
Jalankan ulang notebook analisis untuk memperbarui tabel/grafik turunan.

**Pembanding — 9 November 2026, selesai 15.41 WIB (`highway.other=-2`).**
Pengguna melatih ulang seluruh 12 profil/60 seed-run (13.500.000 episode).
Perubahan config hanya `highway.other`, dari −1 menjadi −2; snapshot, start,
seed, mask endpoint, dan parameter lain sama dengan pembanding 14.31 WIB.
Greedy valid turun **9/60 → 4/60**: A 0/20, B 0/20, C 4/20. Urutan
konfigurasi pada tabel di atas: A=0/0/0/0, B=0/0/0/0, C=1/0/1/2 (per lima seed).
Jumlah keberhasilan C sama, tetapi seed dan rutenya berubah. Kegagalan terbaru:
47 over_distance dan 9 no_available_action, tanpa step_limit (maksimum246 langkah).

Seluruh empat loop valid masih melewati 93,55m service pada way `160013981`
dan `160014008`; penalti lebih besar belum menghindarkan lintasan TPU yang
sedang ditinjau. `other` juga mencakup secondary dan kategori nonprioritas lain,
bukan hanya service. Pada 10.000 episode terakhir per seed, keberhasilan training
horizon-total300k turun B26,45%→6,24% dan C18,37%→13,28%. Seluruh13,5juta baris
history dibaca; ini belum replay penuh Q-table atau validasi lapangan.

**Pembanding — 9 November 2026, selesai 16.10 WIB (secondary `+0.5`, start lama).**
Pengguna menyelesaikan seluruh 12 profil/60 run/13.500.000 episode dengan
`highway.other=-1` dan `secondary_priority=0.5`; parameter lain, snapshot, start,
seed, dan mask endpoint sama dengan pembanding 14.31 WIB. Greedy valid menjadi
**3/60: A2/20, B1/20, C0/20**. Urutan dinamis150k/300k dan horizon-total150k/300k:
A=1/0/0/1, B=0/0/0/1, C=0/0/0/0 (per lima seed). Ada52 over_distance dan5 buntu,
tanpa step_limit; maksimum258 langkah. Dua loop A tanpa service, satu loop B
masih melewati93,55m service pada way160013981/160014008. Keberhasilan training
10.000 episode terakhir horizon-total300k B15,20%/C8,84%, dibanding26,45%/18,37%
pada pembanding −1 tanpa tier secondary.

Pemeriksaan13,5juta history dan60 trace tidak menemukan ketidakcocokan total
langkah/jarak/reward atau lintasan akses terlarang. Reward ruas pada1.062 lintasan
secondary cocok dengan hitungan independen. Ini belum full Q-table replay atau
validasi lapangan. Skor secondary dengan asphalt dan width>=3m adalah0,725;
menaikkan preferensi ruas belum memperbaiki kemampuan menutup loop.

Diagnosis jarak pulang memakai Dijkstra pada graf dengan whitelist way/endpoint,
khusus pemeriksaan hasil; belum menjadi bagian training. Jarak ini mengabaikan
riwayat kunjungan, sehingga merupakan batas bawah optimistis. Contoh B horizon300k
seed0 langkah83: sudah3.519,11m, Haversine ke start1.361,02m, tetapi jalan legal
terpendek2.026,88m. Total minimal5.545,99m sudah melampaui5.500m. Semua57 evaluasi
gagal memasuki kondisi serupa sebelum melewati batas jarak. Reward arah saat ini
tetap Haversine; penggunaan jarak graf untuk reward baru usulan yang perlu dibahas.
Jangan mengubah config/mask atau menjalankan training otomatis. Gunakan metadata
artefak agar hasil3/60 tidak tercampur dengan pembanding4/60 atau9/60.

**Pembanding — 9 November 2026, selesai 16.29 WIB (start depan gang).**
Pengguna memakai input `-7.281518, 112.710178`, snap ke node `311432306`.
Snapshot SHA-256, reward secondary+0,5/other−1, environment, profil dan seed0–4
sama dengan batch16.10 WIB. Greedy valid naik3/60→**38/60 (A0 B18 C20)**.
Urutan dinamis150k/300k dan horizon-total150k/300k: A=0/0/0/0, B=3/5/5/5,
C=5/5/5/5 (per lima seed);12 over_distance/10 buntu. Ringkasan ini membaca
metadata,60 evaluasi dan trace, belum audit seluruh history/replay Q-table baru.
Start lama memiliki1 outgoing edge legal dari2; start baru4 dari4. Jadi koneksi
lokal start berpengaruh besar pada hasil snapshot ini. Testing35/60 pada start
`9645488767` memakai snapshot serta seed acak berbeda; bukan pembanding satu faktor.

Pengguna meninjau bias aturan arah: sebelum2.500m, mendekat mendapat penalti
sedangkan menjauh mendapat0; sesudahnya reward mendekat/penalti menjauh.
Loop dapat memerlukan langkah mendekat di fase awal. Percobaan berikutnya
menonaktifkan penalti awal; Dijkstra tetap diagnosis saja, belum digunakan training.

**Pembanding training — 9 November 2026, selesai 16.54 WIB (outward `0`).**
Input kembali ke `-7.282187, 112.709282`, start `9620836306`; reward secondary+0,5,
return1,20, seed0–4 dan snapshot tetap. Greedy **12/60 (A0 B9 C3)**, dibanding
3/60 pada start yang sama dengan outward0,60. Urutan profil A=0/0/0/0,
B=0/1/3/5, C=0/1/1/1 (per lima seed);40 over_distance/8 buntu, tanpa step_limit,
maks252 langkah. Angka38/60 memakai start berbeda sehingga bukan pembanding satu
faktor. Metadata60 evaluasi/trace diperiksa, belum audit history/full Q replay baru.

Tujuh dari12 loop valid tanpa service. B horizon-total300k berhasil5/5;
seed0/1/3 tanpa service, sedangkan seed2/4 melewati dua way TPU yang ditinjau
(`160013981`, `160014008`). Dari seluruh20 evaluasi per skenario, B5 dan C17
melewati dua way itu; hitungan ini bukan semua lintasan yang sekadar mendekati TPU.
Pemeriksaan konektivitas sementara tanpa dua way tersebut tetap menjangkau24.633
node dari start (semula24.638). Hanya5 node hilang dari jangkauan; ini diagnosis,
bukan perubahan graf/action mask atau pembuktian semua loop bebas TPU tersedia.

Contoh B/C horizon300k seed0:43 langkah pertama sama, kemudian node1720772306
bin9 memilih arah berbeda. B memilih1720772195 (Q3,9216 vs3,4646 untuk9570022203)
dan berhasil5.429,85m tanpa service. C dengan previous1720772406 memilih9570022203
(Q5,7619 vs5,5095 untuk1720772195), melewati93,55m way TPU dan gagal over_distance.
Kedua alternatif legal pada prefix ini. Ini contoh perbedaan kebijakan yang
tersimpan, belum penjelasan lengkap penyebab pembelajaran C. Jangan mengubah
reward/config/training otomatis. Config testing juga outward0, tetapi hasil
terakhirnya35/60 masih batch16.24 WIB dengan outward0,60 dan seed/snapshot berbeda.

Snapshot training memiliki **26.345 node dan 58.020 edge terarah**; start aktif
`311432306` (start checkpoint lama `9620836306`). SHA-256 snapshot:
`f0a5885db46c2f9cd962674aaeaa82f7d5c69579d5ba5d80641f727b6e20f964`.
CSV, Q-table, peta hasil, serta cache merupakan artefak lokal yang tidak masuk
Git. Unduhan OSM baru dapat mengubah graf dan hasil, meskipun seed sama.

**Checkpoint sebelum penalti selisih jarak — 9 November 2026, selesai 21.50 WIB.**

Pengguna menjalankan ulang seluruh A/B/C: 12 profil, 60 seed-run, dan
13.500.000 episode. Metadata konsisten: start311432306 (input−7.281518,
112.710178), snapshot training yang sama, seed0–4, outward0, return1,20,
toleransi20%, 25bin, revisit−10, highway.other−10/missing−2, secondary+0,5.
Penalti selisih jarak belum ada pada batch ini (efektif0).

| Konfigurasi | A valid | B valid | C valid |
|---|---:|---:|---:|
| Dinamis150k | 0/5 | 4/5 | 0/5 |
| Dinamis300k | 1/5 | 4/5 | 0/5 |
| Horizon100k-total150k | 1/5 | 4/5 | 1/5 |
| Horizon100k-total300k | 1/5 | 4/5 | 3/5 |

Total23/60: A3, B16, C4; gagal29 over_distance/8 buntu, tanpa step_limit.
Seluruh kegagalan memiliki closing_action_available=false. Sebanyak13 dari23
loop juga berada pada4.500–5.500m; semua4 loop C di atas5.500m. Ini pengelompokan
jarak rute tersimpan, bukan evaluasi ulang Q-table dengan toleransi10%.
B horizon-total300k: galat rata-rata178,38m, comfort0,9124, overlap0,70% pada
4 loop valid; keberhasilan training10k terakhir63,10%. C konfigurasi sama3/5,
galat766,69m pada3 loop, training10k terakhir22,35%. Rerata subset berhasil
harus dibaca bersama jumlah keberhasilan.

Audit membaca13,5juta baris history, metadata dan60 evaluasi; episode per seed,
epsilon, galat, overshoot dan status konsisten. Ini belum full replay Q-table
atau validasi lapangan. Pembanding B/C21.07 WIB memakai outward0,60: B15/20,
C6/20; batch21.50 memakai0: B16/20,C4/20. Perubahan tidak seragam antar-state.
Testing21.27 WIB tetap batch dua profil B sebelumnya (8/10), snapshot/start/seed
berbeda; tidak dicampurkan ke60 evaluasi training.

## Checkpoint terbaru — 9 November 2026, selesai 22.18 WIB

Penalti galat jarak −5/100 m sudah aktif pada seluruh 12 profil/60 run/13,5 juta
episode, dengan graf/start/reward/environment/seed/akses konsisten antarprofil.
Start 311432306, input −7.281518/112.710178; toleransi 20%, 25 bin, outward 0,
return 1,20, revisit −10, highway.other −10/missing −2/secondary +0,5.

| Konfigurasi | A valid | B valid | C valid |
|---|---:|---:|---:|
| Dinamis 150k | 0/5 | 0/5 | 0/5 |
| Dinamis 300k | 1/5 | 5/5 | 1/5 |
| Horizon 100k-total 150k | 0/5 | 5/5 | 2/5 |
| Horizon 100k-total 300k | 1/5 | 5/5 | 2/5 |

Total **22/60 (A 2 B 15 C 5)**,31 over_distance/7 buntu. Sebanyak 18 dari 22 loop
berada dalam 4.500–5.500 m; ini bukan eksperimen baru dengan toleransi 10%.
Checkpoint 21.50 tanpa penalti menghasilkan 23/60 dan 13 loop dalam rentang tersebut.
B horizon 300k 5/5, galat 161,29 m, comfort 0,9093, overlap 0,71% pada 5 loop,
dan 4 urutan edge unik; B dinamis 150k turun 4/5→0/5. Efek penalti belum merata.

Testing 22.21 WIB: B dinamis 300k 1/5, horizon 300k 5/5, total 6/10. Titik/snapshot/seed
berbeda dari training dan testing sebelumnya; jangan menyimpulkan efek penalti
saja dari perubahan itu. Chat “Jawab pertanyaan umum” melaporkan audit 16,5 juta
history/70 evaluasi; sinkronisasi Markdown mencocokkan source/config/metadata/CSV,
tanpa mengulang audit penuh, tes atau training. Pemilihan konfigurasi final tetap
pada pengguna; lima seed belum membuktikan optimalitas atau validasi lapangan.

Artefak turunan tertinggal: analysis.json training 16.58 masih scoring −1/−0,5;
analysis/training/last_window_summary.csv 21.03 hanya B/C; final-experiment-audit.json
7 November. Perbarui analyze/notebook dari data tersimpan sebelum mengutip
grafik/tabel Bab IV. Pembaruan ini tidak menjalankannya atau mengubah hasil lama.

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

**Show graph nodes** menampilkan node sekitar viewport pada zoom17 ke atas
(maksimum2.000 marker). Klik node biasa/marker akses untuk melihat node tujuan,
key, panjang, serta access way/node tujuan. Garis sian dan label ID menandai
tetangga; oranye menandai node terpilih. Klik ID/marker tetangga untuk berpindah;
tutup popup untuk membersihkan sorotan. Popup jalan dan START juga memiliki
tombol inspeksi node. Daftar memakai outgoing edge graf mentah, termasuk
ruas yang aksesnya ditolak; ini bukan daftar action legal pada suatu step.
Edge paralel tetap terpisah per key, sedangkan jumlah tetangga memakai ID unik.
Peta analisis memakai inspeksi node yang sama.

**Q-values** pada kontrol layer menampilkan nilai Q untuk outgoing action node
terpilih, bukan agregat seluruh state. Aktifkan layer, lalu klik node atau tombol
**Inspect node** pada popup ruas. **Inspect node ID** juga dapat mencari ID tertentu.
Pada rute, pilih **Visit / step** untuk memakai
progress bin dan previous node saat evaluasi; pada node lain pilih **Stored state**.
Popup memakai latar putih dengan opasitas 86% dengan teks tetap solid dan jarak ringkas
agar peta di belakangnya masih terlihat. Detail node/koneksi tetap dapat dibuka.
Popup menampilkan Q per `(next_node, key)`, action terpilih, mask, dan best Q legal
(termasuk tie sesuai toleransi agen). Action tanpa entri memakai `0 · unrecorded`;
state tersimpan tanpa riwayat tidak diberi klaim mask dinamis atau pilihan greedy.
Mask debug direkonstruksi dari prefix trace, config, dan versi akses hasil tersebut;
filter node terbaru tidak dipaksakan pada arsip dengan kebijakan way saja.

Q-table belum dibaca saat membuka peta atau ketika Q-values mati. Endpoint khusus
`/q-values` membaca satu CSV seed secara streaming ketika node diinspeksi dan
mengirim hanya state/action node tersebut beserta konteks kunjungannya. Browser
menyimpan maksimum100 pasangan seed/node per halaman; pindah bin/previous node
memakai data itu. **Reload Q data** membuang cache node terpilih. Hasil di server
tidak dicache, sehingga pembacaan node baru dapat memerlukan jeda; popup mencatat
waktu baca server. Membuka ulang halaman membuang cache browser. Mode ini memerlukan
CLI `serve` terbaru; file CSV/JSON lain tetap tidak dapat diunduh langsung.
Peta dan trace yang berbeda atau Q pengamatan yang tidak cocok ditolak, tanpa
training ulang atau perubahan Q-table/rute/metrik tersimpan.

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
pedestrian, living_street, residential, tertiary. Secondary memakai prioritas
kedua bila `secondary_priority` diisi; service tetap `other`. Surface prioritas: asphalt,
concrete, paving_stones, paved. Width diparse sekali ketika load: angka tunggal,
koma desimal, dan satuan meter didukung; gabungan/invalid mendapat skor missing.
Tag asli tetap tersedia pada CSV/popup. Access scalar kosong/yes/permissive
harus lolos pada way, node asal, dan node tujuan; `foot=yes` tidak override
dan nilai gabungan tidak di-split. Node `access=no`/private/unknown tidak
dapat dilalui, tetapi tetap tersimpan pada graf untuk inspeksi. `barrier=gate`
tanpa pembatasan access tidak otomatis ditolak. CLI menolak start yang
aksesnya tidak lolos tanpa memilih node lain. Penalti permissive tetap
berdasarkan way, bukan node. Metadata baru mencatat
`access_policy_version=edge_and_endpoint_whitelist_v1`. Warna Access jalan
tetap berasal dari tag way. Peta analisis dan hasil training menyediakan
checkbox **Node access / barriers** di panel analisis, tepat di bawah
**Show nodes** (default mati), tanpa checkbox duplikat di kanan bawah.
Pada peta training, gunakan kontrol **Node access** di kanan bawah.
Overlay memuat node bertag access/barrier: merah ditolak whitelist, hijau
diizinkan, abu-abu barrier tanpa access. Popup menampilkan ID, tag asli,
koordinat, serta status aturan node saat HTML dibuat; ini bukan keputusan
lengkap action atau perubahan terhadap rute tersimpan. Legenda menghitung
node bertag pada seluruh graf, terpisah dari jumlah kelompok garis jalan.

Diagram `analisa.ipynb` tetap berbasis edge terarah untuk panjang dan
distribusi/kelengkapan atribut; node dipakai pada ringkasan struktur dan
pemeriksaan koordinat/referensi. Distribusi akses node belum dibuat sebagai
diagram tersendiri.

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
menjalankan training atau memperbarui Q-table. Notebook training berfokus pada
tabel dan grafik; peta hasil dibuka melalui server CLI. Analisis training memisahkan
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

## Perbarui HTML dari hasil tersimpan

Pengaturan workdir hanya memengaruhi tampilan:

```yaml
export:
  map_mode: standard  # standard | advanced; default jika dihilangkan: advanced
```

`testing` memakai `standard`, `training` memakai `advanced`. Keduanya mempertahankan
seluruh seed, metrik, graf sekitar, popup atribut/reward, akses jalan/node, comfort,
penanda KM, pilihan per-KM, panah arah, dan sorotan overlap. `standard` tidak
menyertakan UI/kode pemanggil Q-values. `advanced` menambahkan inspeksi Q-values
melalui CLI server. Leaflet dari CDN dan tile OSM tetap memerlukan internet.
Seluruh graf masih tertanam; mode standard tidak otomatis memperkecil HTML.

Panel informasi peta rute dan analisis memiliki tombol **Minimize / Show details**.
Pada layar HP atau landscape pendek, panel mulai ringkas; saat dibuka tingginya
dibatasi dan isinya bisa digulir. Pilihan seed, KM dan layer tetap tersimpan ketika
panel ditutup. Menu layer rute juga mulai ringkas pada layar kecil; kontrol zoom
berada di kiri bawah. Tata letak memakai tinggi viewport dinamis. Perubahan
template berlaku pada HTML yang dibuat/diekspor berikutnya, tanpa training ulang.

Sesudah training, regenerasi presentasi dari hasil tersimpan:

```bash
go run ./cmd/cli export --workdir workdirs/training
go run ./cmd/cli export --workdir workdirs/testing

# Opsional: perbarui HTML satu profil tersimpan saja.
go run ./cmd/cli export --workdir workdirs/training \
  --profile b-horizon-100000-total-300000

go run ./cmd/cli serve --dir workdirs --port 8080
```

Buka indeks server di `http://127.0.0.1:8080/`, lalu pilih peta hasil yang sama
seperti sebelumnya. Contoh URL langsung:

- Training: `http://127.0.0.1:8080/files/training/output/training/b-horizon-100000-total-300000/routes.html`
- Testing: `http://127.0.0.1:8080/files/testing/output/training/b-horizon-100000-total-300000/routes.html`

Command `export` **menimpa HTML di lokasi aslinya**, tanpa membuat salinan `site/`.
Tanpa `--profile`, peta analisis yang tersedia dan seluruh profil tersimpan
(termasuk seed gagal) diperbarui. Dengan `--profile`, hanya `routes.html` dan
`report.html` profil terpilih yang diperbarui; profil lain serta analisis tetap.
Navigasi All results kembali ke indeks CLI server yang sudah tersedia.

Tidak ada unduhan/training atau perubahan CSV, Q-table, snapshot, YAML maupun
metadata penelitian. Graf, skor, rute dan reward dibaca dari payload HTML lama;
konfigurasi/sumber seed dibaca dari metadata tersimpan, bukan parameter training
YAML saat ini. `export.map_mode` menentukan mode presentasi terbaru. Analisis
lama mempertahankan skor/tanggal lamanya; jalankan `analyze` untuk menghitung
analisis baru. Jangan menjalankan export bersamaan dengan train/analyze yang
sedang menulis hasil workdir yang sama.

Semua HTML disiapkan dan diperiksa dahulu. Kesalahan publikasi atau pembatalan
memulihkan file sebelumnya; ini bukan transaksi tahan crash sistem. Manifest
`<output_dir>/export.json` (schema 2) mencatat pembaruan terakhir: mode, tanggal,
hasil yang diperbarui dan SHA-256 HTML sebelum pembaruan. Metadata eksperimen
asli tetap utuh. Advanced membaca Q-table dari direktori hasil yang sama.

Folder `site/` dari versi export lama dibersihkan setelah ekspor berhasil. Hanya
folder dengan manifest versi lama yang cocok dengan workdir dan struktur file
hasil ekspor yang boleh dihapus. Bila ada file tambahan/symlink atau manifest
tidak cocok, ekspor berhenti tanpa perubahan; pindahkan file tersebut dahulu.
`web-catalog/` tetap terpisah, tidak diubah oleh export. Gunakan `catalog add`
untuk menyiapkan rute pilihan bagi hosting statis/GitHub Pages.

Tombol ←/→ berpindah antar-run tanpa membungkus dari akhir ke awal; tombol dinonaktifkan
pada batas daftar. Dropdown memakai nomor run agar seed panjang tidak terpotong;
seed uint64 lengkap ditampilkan di bawahnya. Buka **seed information** untuk sumber
seed tetap/acak, arti run independen, dan alasan pemilihan representative. Seed
representative bukan jaminan rute terbaik di lapangan. Tautan `routes.html?seed=...`
memilih seed tertentu; **Copy link** pada katalog menyimpan profil dan seed terpilih.

## Download GPX untuk pemeriksaan lapangan

Pada HTML peta rute yang dibuat dengan versi ini, pilih **Training seed** lalu
klik **Download GPX** di sebelah **Fit selected route**. Tersedia pada standard,
advanced, dan peta katalog. GPX dibentuk di browser dari data lintasan tertanam;
tidak ada request endpoint GPX, tambahan library, file server, atau CLI baru.
Nama file memuat label rute/scenario dan seed uint64 utuh.

**Kontrak file GPX:**

- XML UTF-8, GPX1.1, namespace `http://www.topografix.com/GPX/1/1`, atribut
  `version="1.1"` dan `creator="Running Route Q-Learning"`.
- Satu `trk`, satu `trkseg`, berisi `trkpt` dengan atribut `lat`/`lon` WGS84 dalam
  derajat desimal (10 angka di belakang titik, tanpa notasi eksponen).
- Titik pertama adalah node start hasil snapping. Selanjutnya satu titik tujuan
  untuk setiap segmen sesuai urutan langkah; N segmen menghasilkan N+1 titik.
  Kunjungan ulang/arah balik tetap ada. Segmen terputus atau koordinat tidak valid
  ditolak; rute berlabel loop wajib berakhir di node dan koordinat start.
- Seluruh rute seed terpilih diekspor, walaupun tampilan hanya menunjukkan KM
  tertentu atau layer rute disembunyikan. Data tidak disederhanakan untuk GPX.
- Nama/deskripsi mencatat scenario, seed, jarak graf tersimpan, dan alasan
  terminasi; sumber track adalah OpenStreetMap contributors. Teks dibentuk lewat
  DOM XML/`textContent` dan `XMLSerializer`, bukan interpolasi string XML mentah.
- Seed tanpa segmen menonaktifkan tombol. Seed gagal yang mempunyai lintasan
  tetap bisa diekspor untuk inspeksi, dengan status gagal dalam deskripsi.
- Tidak mengisi waktu aktivitas, elevasi, kecepatan, atau rekaman GPS rekaan.
  File ini lintasan rencana. Jarak graf dicatat sebagai teks deskripsi; aplikasi
  penerima dapat menghitung jarak sendiri dari koordinat. Rekaman aktivitas GPS
  di lapangan tetap pembanding terpisah.

Unduhan memakai `Blob` bertipe `application/gpx+xml;charset=utf-8`, object URL,
dan tautan `download`. Object URL dilepas sesudah browser diberi waktu memproses
unduhan. Semua fitur ini bekerja tanpa backend GPX di situs statis.

Implementasi ada di `internal/analysis/routes.gohtml`: `trackPoints`, `buildGPX`,
dan handler tombol `download-gpx`. Ini alat bantu pengguna; tidak mengubah reward,
training, metrik, status validasi lapangan, atau menambah pembahasan skripsi.
HTML lama tidak berubah otomatis. Pengguna akan membuat hasil HTML baru sendiri;
agent tidak menjalankan training, meregenerasi katalog, atau melakukan deployment.
Jika hanya memperbarui presentasi, `cli export` menimpa HTML di lokasi hasil
aslinya tanpa training ulang; folder web-catalog tetap alur terpisah.

Pengujian implementasi memakai HTML pratinjau di /tmp dan unduhan browser nyata:
standard/advanced/katalog, pergantian seed, filter KM, seed kosong, serta lintasan
terputus. File diuji dengan schema XSD resmi memakai `xmllint`; urutan151titik
untuk150segmen, loop, revisits, seed uint64 dan escaping XML diperiksa. GPX filter
KM identik dengan GPX seluruh rute. Ini verifikasi ekspor, bukan validasi lapangan.

Referensi kontrak dan API implementasi:

- [Schema dan dokumentasi GPX1.1](https://www.topografix.com/GPX/1/1/): struktur
  `gpx`/`trk`/`trkseg`/`trkpt`, WGS84, batas latitude/longitude, dan field opsional.
- [Schema XSD GPX1.1 resmi](https://www.topografix.com/GPX/1/1/gpx.xsd): acuan
  validasi file XML, termasuk urutan elemen dan tipe desimal.
- [MDN XMLSerializer.serializeToString](https://developer.mozilla.org/en-US/docs/Web/API/XMLSerializer/serializeToString): serialisasi DOM XML.
- [MDN URL.createObjectURL](https://developer.mozilla.org/en-US/docs/Web/API/URL/createObjectURL_static): URL sementara untuk `Blob`.
- [MDN URL.revokeObjectURL](https://developer.mozilla.org/en-US/docs/Web/API/URL/revokeObjectURL_static): pelepasan object URL setelah digunakan.

## Kandidat validasi lapangan — 12 November 2026

Pengguna memilih **tiga rute dari tiga titik awal berbeda**, seluruhnya dari
`b-horizon-100000-total-300000` (B, 300.000 episode, horizon epsilon100.000).
Katalog lokal menyimpan pilihan berikut secara mandiri dari output testing:

| ID rute | Seed | Jarak graf tersimpan |
|---|---|---:|
| `depan-gang-sebelah-toko9` | `13916397716709765191` | 5.151,87 m |
| `depan-indomaret-dkb` | `16261264761017114444` | 5.298,84 m |
| `depan-uwika` | `9988407548480542851` | 5.527,50 m |

Ketiganya masih `field_validation=not_recorded`: kandidat yang akan diperiksa,
bukan hasil validasi. Catat kesesuaian lintasan/loop, jarak aktivitas GPS dan
selisihnya terhadap graf, akses/hambatan, serta penyimpangan yang diperlukan.
Pertahankan temuan gagal; validasi terbatas pada rute dan waktu pemeriksaan,
bukan generalisasi ke seluruh wilayah. Pengujian ini terpisah dari eksperimen
perbandingan state. Commit katalog bersifat lokal sampai push/deployment.

## Menambahkan rute pilihan ke web-catalog

Alur untuk mencoba lokasi baru sampai publikasi:

1. Ubah `map.latitude`/`map.longitude` (dan radius bila perlu) pada config workdir.
2. Jalankan `go run ./cmd/cli run --workdir workdirs/testing` untuk download → analyze → train.
3. Periksa peta dan metrik hasil; pilih profil serta seed loop yang ingin disimpan.
4. Jalankan `catalog add` di bawah, lalu isi ID dan deskripsi rute.
5. Periksa katalog lokal, kemudian commit/push untuk menjalankan publikasi Pages.

`catalog add` **hanya membuat HTML/detail/metadata dari hasil training tersimpan**;
tidak menjalankan download, analyze atau training ulang. Tidak perlu menjalankan
`export` terlebih dahulu. Tambahkan seed pilihan sebelum hasil sumber ditimpa
percobaan lokasi berikutnya. Command add tidak melakukan commit/push/deployment.

Pilih profil dan **seed aktual** dari hasil testing/training yang sudah kamu
periksa. Ganti `12345` di contoh ini dengan seed dari peta atau metadata hasil:

```bash
go run ./cmd/cli catalog add \
  --workdir workdirs/testing \
  --profile b-horizon-100000-total-300000 \
  --seed 12345
```

CLI kemudian meminta dua input:

```text
Route ID: depan-kampus
Route description: Rute dari depan kampus, melewati permukiman dan jalan utama.
```

ID berupa 1–64 huruf kecil/angka, dengan tanda hubung di tengah bila diperlukan.
ID yang sudah terdaftar meminta input ulang; tidak menimpa rute sebelumnya.
Deskripsi wajib 1–1.000 karakter pada satu baris, ditampilkan di daftar dan peta.
`--dir <folder>` opsional mengganti tujuan; default `web-catalog` relatif terhadap
folder terminal saat command dijalankan. Jalankan dari root proyek untuk default.
Tidak ada config katalog khusus, input koordinat, atau prompt nama rute.

Command membaca HTML dan metadata sumber, memeriksa skenario/seed/graf/start,
metrik evaluasi serta lintasan loop dan jaraknya terhadap aturan yang tercatat.
Hanya seed loop berhasil yang dapat ditambahkan. Parameter YAML sekarang hanya
untuk menemukan output_dir; lokasi/reward/training berasal dari metadata hasil.
Tidak mengunduh data atau melatih ulang. Seluruh graf sekitar, skor, atribut,
popup reward, KM, arah, overlap dan akses node tetap disertakan. Hanya satu seed
pilihan masuk peta katalog; labelnya catalog selection, bukan representative baru.

Katalog **selalu standard**, termasuk bila workdir sumber memakai advanced.
Tidak ada UI/request Q-values atau endpoint metadata. Halaman details.html
menampilkan evaluasi seed pilihan dan konfigurasi sumber; JSON lengkap tetap
menyimpan catatan batch asal. Mode presentasi standard dicatat pada bagian
catalog terpisah tanpa mengganti konfigurasi training asli.

```text
web-catalog/
  index.html                 indeks, pencarian, navigasi dan tautan berbagi
  catalog.json               daftar rute beserta deskripsi dan asal hasil
  .nojekyll
  routes/<id>/
    index.html               peta satu seed pilihan, data graf/rute tertanam
    details.html             informasi lengkap dan metrik seed terpilih
    metadata.json            metadata sumber + identitas katalog/hash HTML sumber
```

Rute lain dipertahankan saat penambahan. Seluruh paket disalin ke staging lalu
dipublikasikan setelah selesai; kesalahan input/validasi/penulisan mempertahankan
katalog sebelumnya. Lock sementara mencegah dua command add menulis bersamaan;
Ctrl+C membatalkan prompt. Snapshot, Q-table dan CSV tidak disalin. Katalog awal
kosong; data uji sintetis tidak dimasukkan ke situs proyek.

Untuk menghapus **satu rute** berdasarkan ID:

```bash
go run ./cmd/cli catalog rm gang-sebelah-toko-9
# Jika memakai folder lain, letakkan flag sebelum ID:
go run ./cmd/cli catalog rm --dir web-catalog gang-sebelah-toko-9
```

`rm` menghapus folder rute pilihan, menghapus entri manifest dan memperbarui
indeks. Rute lain serta hasil training sumber tetap tersimpan. Tidak memerlukan
workdir/seed atau prompt tambahan; ID yang tidak ditemukan menghasilkan error
sebelum ada perubahan. Menghapus rute terakhir menyisakan katalog kosong yang
bisa diisi kembali. Untuk katalog yang rusak/terhapus sebagian, gunakan clear.

Untuk menghapus **semua rute katalog** dan mulai dari katalog kosong:

```bash
go run ./cmd/cli catalog clear
# Untuk katalog di folder lain:
go run ./cmd/cli catalog clear --dir web-catalog
```

`clear` mengganti katalog dengan `index.html`, `catalog.json` berisi daftar kosong,
`.nojekyll`, dan folder `routes/` kosong. Bisa dipakai jika manifest atau peta
terhapus sebagian. Tidak memerlukan workdir/seed; hasil download, analisis dan
training tetap tersimpan. Sesudah clear, jalankan `catalog add` seperti biasa.
Perintah ini langsung menghapus semua pilihan katalog tanpa prompt tambahan.
Folder yang mengandung file di luar struktur katalog atau symlink ditolak.
Add, rm dan clear memakai lock yang sama; pembuatan indeks kosong diselesaikan dahulu
sebelum mengganti katalog lama. Jangan menghapus isi katalog secara manual.
Clear hanya lokal; situs publik berubah setelah commit/push dan deployment.

Untuk melihat situs lokal:

```bash
go run ./cmd/cli serve --dir web-catalog --port 8081
```

Buka `http://127.0.0.1:8081/files/index.html`. Prefix `/files/` hanya milik server
preview Go. Saat dipublikasikan, tautan relatif bekerja di root/subfolder situs.

### GitHub Pages

**Sudah aktif:** [Katalog rute lari](https://davidimannuel.github.io/go-running-route-q-learning/).
Source menggunakan **GitHub Actions**. Pada pemeriksaan 12 November 2026,
indeks publik saat itu menampilkan satu rute pilihan, `depan-gang-sebelah-toko9`.
Katalog lokal kemudian diperbarui menjadi tiga kandidat pada tabel di atas;
status publik berikutnya mengikuti push/deployment, belum diperiksa ulang.
Status ini menunjukkan publikasi berhasil; pemeriksaan lapangan tetap terpisah.

Leaflet berjalan di browser dengan HTML/CSS/JavaScript; backend Go tidak diperlukan.
Leaflet 1.9.4 dimuat lewat HTTPS dari CDN dan tile OSM lewat HTTPS. Attribution
OSM tetap terlihat dan referrer normal dipertahankan. Internet tetap diperlukan;
ini bukan peta offline. Ikuti [panduan Leaflet](https://leafletjs.com/examples/quick-start/)
dan [kebijakan tile OSM](https://operations.osmfoundation.org/policies/tiles/).

Workflow `.github/workflows/catalog-pages.yml` mengikuti
[template statis resmi GitHub](https://github.com/actions/starter-workflows/blob/main/pages/static.yml).
Workflow hanya mengunggah `web-catalog/`, tanpa training/unduhan/build Go di GitHub.

1. Pada repositori GitHub, pilih **Settings → Pages → Source: GitHub Actions**.
2. Commit perubahan kode/workflow dan folder web-catalog yang sudah kamu periksa.
3. Push ke branch `main`; perubahan web-catalog/workflow memicu deployment.
4. Lihat URL hasil pada tab Actions atau halaman Settings → Pages.

Workflow juga dapat dijalankan manual melalui Actions. Proyek dengan GitHub Free
menggunakan repositori publik; lihat [GitHub Pages](https://docs.github.com/en/pages/getting-started-with-github-pages/what-is-github-pages).
Pengaturan di atas sudah dilakukan untuk repository ini. Untuk perubahan
katalog berikutnya, commit/push perubahan `web-catalog/` ke `main`, lalu periksa
workflow **Deploy route catalog to GitHub Pages** di tab Actions. Jika run pertama
gagal dengan `Get Pages site failed`/404 sebelum Pages diaktifkan, pastikan Source
sudah GitHub Actions lalu pilih **Re-run jobs → Re-run all jobs**. Tidak perlu
membuat workflow tambahan lewat tombol Configure pada template GitHub.
File `.nojekyll` dipertahankan; deployment ini langsung mengunggah situs statis
melalui Actions sehingga tidak membutuhkan pemrosesan Jekyll.
Status pemeriksaan lapangan awalnya not_recorded; loop valid di graf bukan bukti
kelayakan lapangan. Pemilihan seed untuk katalog terpisah dari sampel eksperimen
A/B/C yang tetap dilaporkan seluruhnya.

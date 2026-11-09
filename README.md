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

Config penelitian saat ini menggunakan `-7.282187, 112.709282`; testing
menggunakan `-7.2816034, 112.7101655`. Lokasi dan radius masing-masing dapat diubah di config.
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
hasil tersebut. Pencarian `training/` atau `testing/` membatasi workdir;
contoh `training/ b-horizon` juga menyaring profil. Nama workdir yang cocok
tidak dicocokkan ke folder internal `output/training/` atau teks tombol. Peta ditampilkan di panel sebelah daftar sehingga navigasi tetap
tersedia. **Open separately** membuka hasil pada tab lain. Pemilihan hasil
dan **Refresh results** memuat ulang iframe dengan URL baru; server mengirim
HTML tanpa cache. Setelah mengubah kode Go/template server, hentikan `serve`
dengan Ctrl+C lalu jalankan kembali agar server memakai versi kode terbaru.

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
      highway: {weight: 0.55, priority: 1.0, secondary_priority: 0.5, other: -1.0, missing: -0.5}
      surface: {weight: 0.30, priority: 1.0, other: -1.0, missing: -0.5}
      width: {weight: 0.15, priority: 1.0, other: -1.0, missing: -0.5}
      width_threshold_m: 3.0
    return_phase_ratio: 0.50
    outward_penalty_scale: 0.0
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

`highway.secondary_priority` hanya berlaku untuk tag `highway=secondary`.
Kedua workdir menetapkan `0.5`: prioritas utama tetap `+1`, service/kategori lain
`−1`, dan tag kosong `−0.5`. `secondary_link` tetap memakai `other`. Dengan
surface/width kosong, skor gabungan secondary adalah `0.05`, dibanding service
`−0.775`. Aturan ini preferensi penelitian lokal, bukan penilaian keamanan.
Jika parameter dihilangkan atau null, secondary memakai `other` seperti versi
lama; angka `0` merupakan override yang sah. Default core tetap kompatibel dengan
aturan lama. Nilai eksplisit ikut direkam pada config/road_score metadata dan
halaman Recorded settings. CSV, peta analisis, peta rute, dan reward training
memakai pemetaan yang sama; hasil tersimpan tidak berubah sebelum command terkait
dijalankan ulang.

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

**Hasil terbaru training — 9 November 2026, selesai 16.54 WIB (outward `0`).**
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
`9620836306` (start percobaan depan gang `311432306`). SHA-256 snapshot:
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

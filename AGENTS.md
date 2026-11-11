# Repository Guidelines

## Collaboration

Proyek ini menulis ulang core TA rute lari dari Python ke Go. Chat berbahasa Indonesia; identifier, komentar kode, dan error berbahasa Inggris. Pengguna belajar melalui coding dan review per komponen. Diskusikan perubahan desain sebelum implementasi.

**Always keep it simple:** utamakan kode idiomatic, mudah dibaca, standard library, serta penggunaan memori yang wajar. Hindari abstraksi dan format yang belum dibutuhkan. Pertahankan referensi dan komentar singkat. Sinkronkan README dan bantuan CLI; jangan membuat dokumen tambahan tanpa permintaan.

## Project Layout

- `cmd/cli`: run (download → analyze → train), command terpisah, dan penyimpanan hasil.
- `internal/osm`: Overpass, cache, graf mentah, Haversine, parsing width.
- `internal/qlearning`: scoring, state, mask, environment, reward, Q-table, metrik.
- `internal/analysis`: CSV, HTML peta, Q-table, dan report metadata.
- `internal/config`, `internal/logger`, `internal/htmlserver`: konfigurasi, log, navigasi hasil.
- `analysis/`: core analisis Python; notebook hanya parameter, pemanggilan, dan penjelasan.
- `workdirs/`: config, cache, output; tanpa folder input. Output/cache/playground diabaikan Git.

## Research Rules

Acuan baca saja: `/Users/david/widya_kartika/uwika-ta-31123022-2026`. Rujukan teknisnya `rute_lari_core.py` dan `IMPLEMENTASI.md`; analisis final memakai hasil proyek Go sendiri.

Pertahankan target 5.000 m dan kembali ke start; eksperimen aktif memakai toleransi20%, checkpoint lama10%. State A=current node, B=ditambah progress bin, C=ditambah previous node. Action `(next_node,key)` diurutkan numerik. Akses scalar kosong/yes/permissive wajib lolos pada way dan kedua endpoint, tanpa split/list atau override foot. Start yang tidak lolos ditolak tanpa resnap; barrier saja tidak melarang. Penalti permissive tetap dari way. Policy baru `edge_and_endpoint_whitelist_v1` berlaku 9 November; hasil 7 November memakai way saja. Hasil terbaru dapat bercampur versi kebijakan; cek metadata tiap profil. Pertahankan baseline; gunakan output terpisah untuk eksperimen baru. Warna Access jalan tetap tag way; overlay Node access default mati pada kedua peta, memakai tag access/barrier node dan status whitelist dari Go. Akses node di peta analisis memakai satu checkbox Node access / barriers di bawah Show nodes; tanpa duplikat kanan bawah. Server/iframe memuat ulang HTML tanpa cache. Legenda menghitung node bertag. Diagram analisa tetap edge; node untuk struktur/validasi CSV.

Graf tetap mentah dan bidirectional; simplifikasi di luar lingkup saat ini. Seleksi komponen/truncation belum diterapkan. Jarak memakai Haversine; hasil tidak identik dengan graf proyeksi/simplifikasi atau RNG Python. Overlap adalah hitungan lintasan ruas fisik berdasarkan endpoint, menggabungkan arah balik/geometri identik. Jalan berbeda yang berdekatan bukan overlap. Skor highway/surface/width adalah preferensi penelitian, bukan keamanan aktual.

## Final Experiment

Workdir `training`: A/B/C × dinamis150k/300k dan horizon100k-total150k/300k, 12 profil/60 training/13.500.000 episode. Seed tetap0–4, alpha0.15, gamma0.95, epsilon1→0.05, evaluasi greedy0. Setiap profil/seed mulai dari Q-table/RNG mandiri. `testing`: dua profil B300k aktif (dinamis/horizon100k), koordinat sendiri dan lima seed acak bersama per command; total3.000.000episode. Nama profil kedua workdir identik; jangan memakai alias lama a/b/c.

Parameter aktif: toleransi20%, 25 bin, max_steps500, max_workers5, outward0, return1.20, revisit−10; highway.other−10/missing−2. Penalti selisih jarak akhir−5/100m sudah dilatih pada batch22.18 WIB (22/60 valid). Reward lain ada di YAML. Jangan menambah variasi atau menjalankan training panjang tanpa permintaan. Pengguna masih mengevaluasi konfigurasi; jangan menetapkan final otomatis. Prioritas berikutnya analisis BAB IV dan penyesuaian Bab I–III.

Pengguna menyelesaikan hasil final 7 November 2026 pukul18.25 WIB: greedy31/60, A1/20, B18/20, C12/20. Urutan dinamis150k/300k dan horizon-total150k/300k: A=0/1/0/0, B=5/5/3/5, C=0/3/4/5, masing-masing dari5. Angka ini checkpoint historis 7 November; output terbaru dapat sudah dilatih ulang. Tabel ringkasan ada di README; sumber angka terkini tetap CSV/metadata tiap profil.

Audit7 November (sebelum mask node): seluruh13,5juta history diperiksa;60 evaluasi dan8.640 baris trace direproduksi dari Q-table byte-for-byte. Semua29 gagal tanpa closing legal:25 over_distance/4 buntu; tidak ada step_limit, maksimum353 langkah. Ringkasan lokal: `workdirs/training/output/analysis/final-experiment-audit.json`. Output pendahuluan/arsip lama dihapus atas permintaan pengguna; hasil final sekarang dipertahankan.

Pembanding **training9 November, selesai14.31 WIB (`highway.other=-1`)** memakai mask endpoint pada seluruh12 profil: **9/60 greedy valid, A0/20 B5/20 C4/20**. Urutan dinamis150k/300k dan horizon-total150k/300k: A=0/0/0/0, B=2/0/1/2, C=1/0/1/2 (per5seed). Gagal44 over_distance/7 buntu, tanpa step_limit; maksimum285 langkah. Sembilan loop/delapan urutan edge unik; overlap valid B5,33%(n5), C4,09%(n4). Snapshot/reward/start/seed konsisten;13,5juta history dibaca,60 metrik cocok dengan trace, tanpa node/way terlarang. Belum full Q-table replay baru/validasi lapangan. Ringkasan/diagram `output/analysis` bertanggal7 November masih lama; jangan memakainya untuk angka batch baru atau mencampurkan testing.

Pembanding **9 November15.41 WIB**, hanya `highway.other` −1→−2: seluruh12 profil/60 run/13,5juta episode selesai, **4/60 greedy valid (A0 B0 C4)**, dibanding9/60 sebelumnya. Urutan empat konfigurasi: C1/0/1/2, A/B semua0 (per5seed); seed/rute C berubah. Gagal47 over_distance/9 buntu, tanpa step_limit, maks246 langkah. Semua4 loop masih melewati93,55m service pada way160013981/160014008. Penalti `other` juga mengenai secondary, bukan khusus service. Tail10k horizon300k turun B26,45→6,24%, C18,37→13,28%. History13,5juta dibaca; belum full Q replay/validasi lapangan. Batch ini memakai−2; config aktif sesudahnya dijelaskan berikut. Jangan menjalankan training otomatis. Artefak analisis yang belum diperbarui dapat masih memakai−1. Angka14.31/7November adalah pembanding, bukan output terkini.

Keputusan9 November berikutnya sudah diterapkan: pengguna mengembalikan `highway.other=-1`; training/testing menambahkan `highway.secondary_priority=0.5`, khusus secondary (bukan secondary_link). Kelompok utama tetap+1, service/other−1; parameter lain tetap. Omitted/null memakai Other untuk kompatibilitas config/metadata lama; nol valid. Nilai masuk metadata dan pemetaan scoring bersama untuk training/CSV/peta. Hasil4/60 tetap eksperimen−2; hasil tier baru ada berikut. Jalankan analyze untuk memperbarui peta/CSV setelah mengubah scoring; jangan menafsirkan artefak lama sebagai hasil aturan baru.

Pembanding **9 November16.10 WIB**, secondary+0,5/other−1:12 profil/60run/13,5juta episode, **3/60 greedy valid (A2 B1 C0)**. Urutan empat konfigurasi A1/0/0/1, B0/0/0/1, Csemua0. Gagal52 over_distance/5 buntu, tanpa step_limit, maks258 langkah. Dua loop A tanpa service; B masih93,55m service TPU. Tail10k horizon300k B15,20%/C8,84%. History dibaca;60 trace cocok total/jarak/reward/akses,1.062 lintasan secondary cocok hitungan skor. Belum full Q replay. Diagnosis Dijkstra graf legal (mengabaikan revisit, hanya batas bawah) menunjukkan57 gagal sudah tidak cukup anggaran pulang sebelum5.500m. Contoh B horizon300k seed0 step83:3.519m terpakai+2.027m jarak jalan, sementara Haversine1.361m. Reward arah tetap Haversine; memakai jarak graf baru usulan, jangan implementasi/training otomatis. Config tier baru dipertahankan, bukan bukti perbaikan hasil.

Pembanding **training9 November16.29 WIB**, input−7.281518/112.710178, start311432306: **38/60 valid (A0 B18 C20)**; urutan profil A0/0/0/0 B3/5/5/5 C5/5/5/5. Snapshot/reward/env/profil/seed sama dengan batch16.10 (3/60); hanya start berubah. Start lama1/2 edge legal, baru4/4. Angka dari metadata/60 evaluasi/trace, belum audit history/Q replay baru. Testing35/60 memakai start9645488767, snapshot/seed berbeda. Pengguna mencurigai bias radial fase awal; outward adalah penalti mendekat, bukan bonus menjauh. Batch ini masih outward0,60; percobaan berikutnya memakai0, return tetap1,20. Bahas reward arah dulu; jangan otomatis memakai Dijkstra atau mengubah config/training. Koordinat kedua workdir adalah edit pengguna, pertahankan.


Pembanding **training9 November16.54 WIB**, outward0: start kembali9620836306 (−7.282187/112.709282), snapshot/reward lain/seed tetap. **12/60 valid (A0 B9 C3)**, urutan Asemua0 B0/1/3/5 C0/1/1/1. Bandingkan3/60 pada start sama/outward0,60;38/60 berbeda start. Gagal40 over_distance/8 buntu, maks252 langkah tanpa step_limit. Tujuh loop tanpa service; B horizon300k5/5, seed0/1/3 tanpa service. Dua way TPU160013981/160014008 dilintasi B5/20 C17/20; diagnosis tanpa dua way hanya mengurangi reachable24.638→24.633, bukan memaksa semua loop lewat TPU. Contoh B/C horizon300k seed0 bercabang pada step44/node1720772306; dua alternatif legal, C memilih9570022203 Q5,7619 vs5,5095 ke1720772195; B memilih1720772195 dan berhasil tanpa service. Metadata/60 trace diperiksa, belum history/full replay. Jangan auto-adjust/training. Testing YAMLoutward0 tetapi hasil35/60 masih batchoutward0,60; koordinat pengguna dipertahankan.

## Checkpoint 9 November 21.50 WIB dan penalti galat jarak

Pengguna melatih ulang12 profil A/B/C/60run/13,5juta episode; seluruh metadata
outward0/toleransi20%/25bin/start311432306/seed0–4 konsisten. Greedy23/60:
A3 (0/1/1/1), B16 (4/4/4/4), C4 (0/0/1/3), urutan konfigurasi tetap.
Gagal29 over_distance/8 buntu; tidak ada step_limit atau closing legal pada
kegagalan. Dari23 loop,13 masuk rentang10%; semua4 loop C>5.500m.
B horizon300k:4/5, galat178,38m/comfort0,9124/overlap0,70% pada4loop;
tail10k63,10%. C horizon300k3/5, galat766,69m pada3loop/tail22,35%.
History13,5juta/metadata/evaluasi diperiksa; belum full Q replay/validasi lapangan.
Testing21.27 WIB tetap8/10 pada dua profil B dengan snapshot/start/seed berbeda.

Keputusan pengguna sesudah audit: pertahankan terminasi over_distance dan mask
closing; tambahkan distance_error_penalty_per_100m=−5 di kedua YAML.
Rumus koefisien*abs(jarak_akhir−target)/100, proporsional, sekali pada seluruh
terminasi (termasuk buntu awal/setelah bergerak). Langkah nonterminal0; jangan
menghukum kekurangan jarak setiap langkah. Tambahan terpisah DistanceError /
reward_distance_error masuk trace/popup dan jumlah reward sebelum Bellman;
terminal tanpa bootstrap. Nol/omitted menjaga hasil lama; konfigurasi baru
memakai reward_scheme=configurable_signed_unclipped_distance_error_v2.
Batas valid, fase, revisit, state, seed dan reward lainnya tetap. Angka23/60
adalah checkpoint sebelum penalti; hasil sesudah−5 ada di bagian berikut.
Hasil/snapshot/notebook output lama dipertahankan. Gunakan output_dir berbeda
serta --snapshot untuk eksperimen berikut agar pembanding tidak tertimpa.

## Hasil terbaru 9 November 22.18 WIB — penalti galat sudah aktif

Batchtraining 12 profil/60 run/13,5 juta episode selesai dengan −5/100 m:
**22/60 valid (A 2 B 15 C 5)**; urutan konfigurasi A 0/1/0/1, B 0/5/5/5, C 0/1/2/2.
Gagal 31 over_distance/7 buntu. Sebanyak 18 loop pada 4.500–5.500 m, dibanding 13/23
checkpoint 21.50; ini pengelompokan hasil toleransi 20%, bukan uji 10% baru.
B horizon 300k 5/5, galat 161,29 m/comfort 0,9093/overlap 0,71% pada 5 loop,
4 urutan edge unik. B dinamis 150k turun 4/5→0/5; manfaat penalti belum merata.
Start 311432306, input −7.281518/112.710178; snapshot/env/reward/seed 0–4
konsisten antarprofil. Testing 22.21 WIB:6/10 (B dinamis 300k 1/5, horizon 5/5),
start 311431485/input −7.281885/112.711476; snapshot/seed berbeda.
Chat “Jawab pertanyaan umum” melaporkan audit 16,5 juta history/70 evaluasi;
sinkronisasi ini hanya mencocokkan source/config/metadata/CSV/rute.
Analysis.jsontraining 16.58 masih highway.other −1/missing −0,5;
last_window_summary.csv 21.03 hanya B/C; audit 7 November historis. Perbarui
artefak dari hasil tersimpan sebelum tabel/grafik Bab IV, tanpa training ulang.
DOCX tetap milik pengguna; verifikasi/validasi ditunda. Config terbaru belum
otomatis menjadi konfigurasi final penelitian.

## Maps and Validation

Peta mempertahankan seed, metrik, marker input/start/finish/KM, serta layer Comfort/Access. Klik node biasa/akses membuka outgoing neighbors (ID/key/panjang/access) dan sorotan koneksi; semua edge mentah termasuk yang ditolak akses, bukan mask episode. Show graph nodes pada rute memakai zoom>=17/viewport/maks2.000 marker; popup jalan/START juga menyediakan inspeksi. Tutup popup membersihkan sorotan. Route distance memilih seluruh rute/per-KM; konteks lain samar. Sorotan ruas ulang dan panah arah default mati. Popup memilih setiap traversal beserta step/interval/reward. Pembaruan 9 November menambahkan tag/status node pada payload peta tersimpan; rute, metrik, skor edge, dan artefak training tetap. Jangan menganggap rute baseline dilatih ulang dengan mask node. Desktop/mobile diuji.

Q-values pada peta rute default mati; hanya outgoing action node/state terpilih, per seed. Endpoint `/q-values` streaming satu CSV, browser cache maks100 node/seed; bukan embedding Q-table. Popup memilih kunjungan trace (mask debug dari prefix/config/policy) atau state tersimpan (mask dinamis tidak diketahui). Bedakan nol tersimpan/unrecorded, key paralel, tie legal, dan chosen. Versi akses arsip tetap dihormati; trace/HTML/Q yang tidak cocok ditolak. Jangan mengubah policy/Q/rute demi visualisasi.

Jangan menyimpulkan optimalitas, generalisasi, atau validasi lapangan dari lima seed. Laporkan kegagalan dan jumlah sampel saat memakai rerata rute valid. Overlap rendah belum menjamin bentuk rute yang memuaskan. Jangan memakai angka arsip sebagai hasil baru atau mengubah metadata lama.

Gunakan gofmt, `go test ./...`, `go vet ./...`, `go build ./...`; tambah `go test -race ./...` untuk perubahan konkurensi. Tes memakai fixture lokal dan expected Q/reward independen. Setelah perubahan Python, jalankan notebook terkait dari kernel bersih dan periksa grafik. Commit deskriptif; simpan notebook tanpa output dan jangan sertakan artefak lokal. Baca ulang file sebelum edit untuk menjaga perubahan sesi lain.

## HTML export and seed navigation — 11 November

`export.map_mode`: standard/advanced, omitted defaults advanced. Testing standard;
training advanced. Standard retains all embedded graph/layers/metrics/reward popups,
without Q inspection code or backend requests. Advanced uses CLI `/q-values`.
`cli export --workdir ... [--profile ...]` regenerates `output/site/` from saved
HTML payloads and recorded metadata, never today's training parameters. No training,
source changes, or raw CSV/Q/YAML copying. Whole site replacement is staged; a
selected profile replaces the package with only that profile. Preserve failed seeds
and old analysis dates/scores. `export.json` records source hashes and export mode.
Relative navigation/report/index is static; Leaflet CDN and OSM tiles remain external.
Advanced site QSource resolves to the original result; serve a common parent root.
Seed arrows stop at boundaries; full uint64 seed stays text. Source fixed/random
comes from recorded automatic_seeds, not current YAML. Seed links and catalog Copy
link preserve selection. Existing outputs are not regenerated automatically.

## Curated web catalog — 12 November

`cli catalog add --workdir ... --profile ... --seed ... [--dir web-catalog]`
prompts for ID and description (no route-name/coordinate/default-config prompt).
Uses saved HTML/metadata; only a matching successful closed loop within recorded
bounds can be added. Current YAML locates output_dir only. Catalog is ALWAYS
standard, regardless of source mode. Full graph/map features remain, one selected
seed, no Q inspector. Selection is labeled catalog selection, not representative.
`web-catalog/` is tracked: root index/catalog.json/.nojekyll; routes/<id>/index.html,
details.html,metadata.json. Metadata retains source batch/config and adds catalog
provenance/hash/mode separately; report table filters selected seed. Never copy raw
Q/CSV/OSM or run training. Interactive validation retries duplicate IDs; additions
stage/copy previous catalog and publish as a batch under an exclusive sibling lock.
Ctrl+C cancels input. Catalog starts empty; synthetic previews stay in /tmp.
Pages workflow uploads web-catalog only, on main/path changes or manual dispatch.
User must choose GitHub Actions as Pages source and push; do not claim deployment
or field validation. All internal links relative; HTTPS Leaflet/OSM, normal referrer.
Preview with serve --dir web-catalog --port 8081, then /files/index.html.
Route/analysis panels use native Minimize/Show details; mobile/short landscape
starts collapsed, scrollable at <=55dvh when open. Seed/KM/layer state persists.
Mobile route layers collapse; zoom bottom-left; fit/popup padding measures panel.
Template changes require HTML regeneration, never research retraining.
Workflow: edit map coordinates → cli run → inspect/select saved profile/seed →
catalog add → preview → user commit/push. Add only generates presentation from
saved results; no download/analyze/train or prerequisite export. Add chosen routes
before the next experiment overwrites their source results.
`cli catalog rm [--dir web-catalog] [--debug] <id>` removes one selected route,
updates index/manifest, keeps other routes and training results. Flags precede ID;
no extra prompt. Unknown IDs fail unchanged; removing the last route leaves a
usable empty site. Uses shared add/clear lock, directory validation and staging.
`cli catalog clear [--dir web-catalog] [--debug]` resets all selected routes to
an empty index/manifest/.nojekyll/routes, even if files are partly deleted. No
extra prompt/workdir; source results remain. Shares add lock and stages publication;
rejects symlinks/unexpected paths to protect non-catalog data. User requested and
ran reset on12November, then added one route again: depan-gang-sebelah-toko9,
seed12226258565145578727/profile B horizon100k-total300k. The checked-in catalog
contains this selected result; original workdir outputs remain ignored. Source
field_validation is not_recorded. No push/deployment performed.

## Combined workflow — 12 November

`cli run --workdir ... [--profile ...] [--force] [--debug]` validates training
config/profile first, then reuses download → analyze → train sequentially.
Always prepares the snapshot for the current map query, using its cache unless
forced. Stops on failure/cancellation; completed stages remain saved. No snapshot
override, notebook, export or catalog step. Do not edit config during a run.
Existing outputs follow standalone overwrite rules; unselected profiles remain.
No live download or research training was run when adding this command.

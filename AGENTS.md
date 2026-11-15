# Repository Guidelines

## Real OSM way Haversine evidence — 15 November 2026

Added internal/osm/verification_haversine_test.go and testdata/way_533112673.json
on user request. Three original nodes/one complete way from training snapshot
d2402df59465909a6dbe85c7f860dbd6cd3789d375b09ce3642694e18453ff66, not synthetic.
TestVerificationWay533112673Haversine runs Load -> Graph.LoadOSM, then checks
all four edge lengths against independent references (absolute 1e-6 m) and
production GreatCircleDistanceM. Logs node coordinates, R6371009, lengths
51.764378387238/161.293877105247 m (reverse same) and forward sum213.058255492484.
README has exact verbose uncached command. Fixture local, no download/output
dependency. All tests/vet/build and diff-check pass. Existing production Go,
YAML, OSM/CSV sources unchanged by hash. No training/reward/results/catalog/Word
changes, commit or push. Actual log and independent calculation manifest kept in
TA docs/assets/bab_3_go/subbab_3_3_2_lengkap_20261115/bukti_haversine/.
This checks model distances, not field accuracy. Do not expand log/test scope
without need; user wants concise evidence for thesis screenshots.

## Perbandingan training dengan backup — 15 November 2026

Pengguna melatih ulang workdir training dan menyimpan pembanding pada
`workdirs/training/output-BU-20261115`. Seluruh 465 nama berkas cocok.
Perbandingan SHA-256 seluruh isi berkas: 365 identik dan 100 berbeda.
Semua 240 CSV identik byte demi byte, termasuk 12 history (13.500.000 episode),
60 Q-table, 12 evaluasi profil, dan 60 trace evaluasi seed. Dua belas peta rute,
OSM mentah, nodes.csv, dan edges.csv juga identik. Parameter reward,
environment, state, seed, dan metrik pada metadata tidak berubah.

Seratus perbedaan hanya waktu proses: 87 JSON berbeda pada started_at dan
completed_at; 13 HTML identik setelah nilai timestamp proses disisihkan.
Jadi hasil training tersimpan sama persis untuk 12 profil dan 60 run yang
ada; seluruh folder tidak identik byte karena waktu run berbeda. Jangan
menghapus atau menyamakan timestamp untuk menghilangkan perbedaan audit.
Bukti hash dan rincian:
`docs/assets/sinkronisasi_go/perbandingan_output_training_20261115.json`
pada repositori TA. Audit membaca output, tanpa training ulang atau perubahan
kode, konfigurasi, backup, maupun hasil. Analisis/grafik yang sudah ada ikut
cocok dengan backup; kesamaan itu tidak berarti semuanya baru dibuat ulang.

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

Workdir `training`: A/B/C × dinamis150k/300k dan horizon100k-total150k/300k, 12 profil/60 training/13.500.000 episode. Seed tetap0–4, alpha0.15, gamma0.95, epsilon1→0.05, evaluasi greedy0. Setiap profil/seed mulai dari Q-table/RNG mandiri. `testing`: satu profil B horizon100k-total300k aktif, koordinat sendiri dan lima seed acak per command; total1.500.000episode. Nama profil kedua workdir identik; jangan memakai alias lama a/b/c.

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
`cli export --workdir ... [--profile ...]` now updates original maps/reports IN
PLACE (12 November); no duplicate output/site or new static index. All results
uses the CLI server index; advanced Q uses the map's own location. Profile flag
updates only that profile; otherwise refresh saved analysis and all profiles.
Use embedded payload/recorded metadata, never today's training parameters. CSV,
Q-tables, metadata, YAML and snapshots stay byte-for-byte intact. Prepare all HTML
first, roll back ordinary publish failures/cancellation. output/export.json schema2
records latest refresh/source hashes. No crash-atomic guarantee or concurrent
train/analyze/export on one workdir. A verified legacy site/schema1 is removed;
unknown files, symlinks or mismatched manifests abort unchanged. The old testing
site (8 generated files) was removed on user request; original HTML awaits the
user's export command. Public catalog is separate and untouched. Leaflet CDN and OSM tiles remain external.
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
Ctrl+C cancels input. Catalog was initially empty; synthetic previews stay in /tmp.
Pages workflow uploads web-catalog only, on main/path changes or manual dispatch.
Pages is enabled with GitHub Actions. Public site verified12November2026:
[Running route catalog](https://davidimannuel.github.io/go-running-route-q-learning/).
The public index had one route at that check; the local catalog now has three
selected candidates pending the next push/deployment. Publication is not field validation.
All internal links relative; HTTPS Leaflet/OSM, normal referrer.
Preview with serve --dir web-catalog --port 8081, then /files/index.html.
Route/analysis panels use native Minimize/Show details; mobile/short landscape
starts collapsed, scrollable at <=55dvh when open. Seed/KM/layer state persists.
Mobile route layers collapse; zoom bottom-left; fit/popup padding measures panel.
Template changes require HTML regeneration, never research retraining.
Route maps now include browser-only Download GPX for the selected seed, full
traversal regardless of KM filter. GPX1.1 WGS84 track, N+1 points; preserve revisits,
no fabricated time/elevation. Empty seed disables export; disconnected data fails.
Contract/API references live in README Go. This is a utility outside thesis scope;
user refreshes HTML with cli export; retraining is unnecessary. Do not regenerate outputs or publish automatically.
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
seed12226258565145578727/profile B horizon100k-total300k at that checkpoint. That
selection has since been replaced by the three candidates below; original workdir outputs remain ignored. Source
field_validation is not_recorded. User pushed and deployed successfully after
the initial Pages404 was resolved by enabling Actions and rerunning the workflow.
Future web-catalog changes require commit/push; verify Actions and the live site.
No automatic commit/push is implied by editing local catalog files.

## Field validation candidates — 12 November

User chose THREE routes from THREE distinct starts, not five. Local catalog IDs:
depan-gang-sebelah-toko9 (seed13916397716709765191,5151.87m),
depan-indomaret-dkb (seed16261264761017114444,5298.84m),
depan-wk (seed9988407548480542851,5527.50m). All B/horizon100k/total300k,
matching effective Q-learning config; standard maps include GPX. All remain
field_validation=not_recorded. Check loop, GPS-vs-graph distance, access/hazards
and deviations; retain failed field checks. This is limited case validation,
separate from the state-comparison experiment. Do not claim fieldwork completed.

## Combined workflow — 12 November

`cli run --workdir ... [--profile ...] [--force] [--debug]` validates training
config/profile first, then reuses download → analyze → train sequentially.
Always prepares the snapshot for the current map query, using its cache unless
forced. Stops on failure/cancellation; completed stages remain saved. No snapshot
override, notebook, export or catalog step. Do not edit config during a run.
Existing outputs follow standalone overwrite rules; unselected profiles remain.
No live download or research training was run when adding this command.


## Road score distribution in graph notebook — 13 November

analisa.ipynb section9 calls GraphAnalysis.show_score_distribution(); section10
saves exports. Read original finite comfort_score from Go edges.csv, never
rescore using current YAML. Group positive/zero/negative across all directed
edges before access/movement filters; keep zero-count categories. Include min/max,
Indonesian chart labels and CSV/PNG exports road_score_groups,
road_score_statistics and road_score_distribution. This is not route reward,
legal action count or proof of field safety. No new dependencies/training/download.
Notebook stays thin and saved without outputs; execute a temporary copy from a
clean kernel for QA. Preserve user WORKDIR/display settings and training notebook.


## One road score figure with numeric intervals — 13 November

User replaces separate sign table/chart with one histogram: width0.25 is display
only, counts per bin, full numeric range and sign totals/percentages in legend.
Keep empty bins; left-closed/right-open except last closed both ends. Zero remains
its own sign group even when stacked in the first nonnegative bin. Export the
additional road_score_intervals.csv; do not round/clip/recompute original scores.
Retain sign/range CSVs for data provenance, without a duplicate displayed table.

## Verification discussion reset — 14 November 2026

User requested removal of the newly added verification tests and wants to discuss
verification again from the beginning, one component at a time. Removed only
internal/qlearning/verification_test.go and testdata/verification_graph.json.
Existing unit tests, production code, notebooks, config and research results stay.
The previous seven-node fixture and its 26 passing cases are historical only;
do not recreate them or treat them as the approved current design. Thesis assets
and temporary Word remain historical references; no document edits requested.
Discuss the graph and checks before implementing replacement tests.

## First verification case reimplemented — 14 November 2026

User approved only a single normal-step test, with readable source and actual
verbose logs for screenshots. Added Graph.AddRoad(from,to,lengthM,tags) in
internal/osm/graph.go; creates missing nodes without coordinates, clones tags,
parses width, assigns WayID, and reuses addEdge for bidirectional/parallel keys.
LoadOSM path is unchanged. Use NewGraph and finish inserts before preparation.

internal/qlearning/verification_test.go now contains only TestVerificationNormalStep.
Direct S1–A2–B3 construction, two roads1000m, residential/asphalt/width3/accessyes.
Production TrainStep with forced S→A (no policy sampling), initial Q2/nextQ4,
alpha.15/gamma.95 gives reward10/Q3.77. Assertions check actual state/distance,
all reward components, running flag, bootstrap and persisted Q, tolerance1e-9.
Logs print source attributes/components/manual equation/actual values, for screenshots.
Explicit synthetic radial map0/1000/2000 is assigned only in test after preparation.
No real coordinates, Haversine/direction/overlap/termination verification claimed;
coincident default coordinates must not be used for future geographic metrics.
Old seven-node/26-case suite stays removed. Do not add further cases until discussed.
No Word edits, downloads or research training. README has the exact command.

## Verification simplified to a single environment move — 14 November 2026

User found the three-node/Q example too complex. TestVerificationNormalStep now
has only nodes1/2, one AddRoad of1000m, and one env.Step(ActionID{2,0}).
Logs show current/next node, accumulated distance, and reward10; assertions check
transition/distance/reward components. DefaultConfig prepares the minimal example;
no claim it mirrors the full current experiment. No Q-table/TrainStep/RNG/manual
radial map remains. Default node coordinates are unused for this step's reward.
No direction/Haversine/overlap/termination/policy/Bellman verification claimed.
In particular env.Step does not execute TrainStep's post-move dead-end handling.
Keep further changes incremental; no Word or research training requested.

## Active defaults aligned — 14 November 2026

User requested DefaultConfig match actual workdir settings. Shared defaults now
use tolerance20%,25bins,max500steps,5workers; highwayOther-10/Missing-2/secondary.5,
outward0/return1.2/finalError-5. Other rewards and alpha/gamma/epsilon unchanged.
Episode profiles, decay durations and seed policy still come from YAML; generic
unprofiled fallback Episodes10000 is unchanged, not a research profile. YAML files,
notebooks, saved results and catalog were not regenerated or edited.

Old numeric unit cases updated for6000m upper/4000m lower,25bins and new reward
values. Python reference fixture remains byte-identical and is checked with its
explicit historical highway settings, not today's defaults. Archived reward JSON
is decoded into zero structs (missing secondary=nil=>recordedOther, error=0);
new YAML omission uses current defaults and explicit0/null retain override meaning.
ReferenceWorkdirConfigurations now checks full environment/reward/default shared
training values against active YAML. Full go test ./... passed (8packages).
VerificationNormalStep stays the same two-node env.Step test, with reward10;
no Q setup or extra verification flow. No Word edits/research training/download.

## Reusable graph and detailed single-step log — 14 November 2026

User requested detailed manual-verification logs for1→2 and a reusable builder.
New test-only synthetic_graph_test.go defines buildSyntheticGraph(t)*osm.Graph:
fresh7nodes/9bidirectionalroads(18edges), explicit lengths/tags via AddRoad,
2→3key0tertiary/key1residential,3–4permissive,2–4private,endpoint7private,
3–6service branch,5–1missing highway/surface/width. Matches the numeric full-graph
PlantUML. No global shared mutable fixture, JSON, loader or production code edits.

VerificationNormalStep now calls the builder, still one direct env.Step1→2 with
DefaultConfig/ScenarioA. Verbose logs include actual config, states, cumulative
distance/visits, legal actions, edge/tag/width, attribute scores/weighted terms,
cached score, length factor, each reward component and why zero, total/status.
Independent expected scores1/reward10 and actual tags/state/distance/components
are asserted. No Q/policy or additional verification cases added. Full tests,
vet and build pass. Default raw node coordinates still zero; do not claim
Haversine/return-direction/geometry-overlap coverage. Future coordinate/radial
setups must be discussed before those checks. Word/results/YAML unchanged.

## Concise Q-value verification log — 14 November 2026

User found detailed reward logs too verbose and approved a minimal Bellman check.
They specifically want one label "Q value", no "Q manual" or "Q program" lines;
manual calculations belong in Word later, and no Word edits are requested now.
VerificationNormalStep now uses production TrainStep for explicit1→2/key0,
DefaultConfig/ScenarioA, assigned Qbefore2 and next legal Q(3,0)=4/(3,1)=1.
Epsilon0; non-nil fixed RNG required by API but policy sampling is bypassed.
Shared buildSyntheticGraph is unchanged. Result reward10/maxnext4/QAfter3.77.
Verbose output prints states/action, distance/reward, alpha/gamma/Qbefore,
next-action Qs, terminal/maxnext, and one Q value expression interpolated from
actual StepResult. No separate manual/program result or reward breakdown log.
Independent assertion expects3.77 with1e-9 tolerance, checks actual stored Q,
state/action/distance/reward/Qinputs and flags. Full tests/vet/build passed.
No production/config/graph/notebook/results/Word changes. Still just one case;
no terminal-Q/policy/Haversine/direction/geometry claims or extra cases.

## Reward calculation added to the Q log — 14 November 2026

User now requests detailed reward calculations in the same single-step test.
VerificationNormalStep logs source tags, attribute scores/width threshold,
weighted road score, length-scaled edge reward, access/direction/revisit/terminal/
no-action/distance-error components with reasons for zero, and total reward10.
One Q value expression remains unchanged; no Q manual/Q program labels or Word
edits. Added independent assertion that three scores and cached road score are1.
Shared graph/defaults/production code unchanged. Full tests/vet/build pass;
actual log has reward10 and Q3.77. No extra cases/training/downloads.

## Reward log limited to three lines — 14 November 2026

User explicitly wants only Road score, Road reward and Reward total in addition
to the core Bellman log. Removed individual tags/attribute scores/zero-penalty
explanations and their unused variables. All assertions, TrainStep, Q setup and
shared graph unchanged. Do not expand verbosity again without user request.

## Five selected log lines and varied synthetic lengths — 14 November 2026

User chose EXACTLY five lines: State/action/nextstate, Road score, Road reward,
Q before/alpha/gamma, Q value. Removed Distance/Reward total/next-action/Terminal
log lines; assertions still verify nonterminal status, max legal Q4 and zero other
reward components. Do not add logs without request. Q manual/Q program remain absent.

User requested varied non-round meter lengths in buildSyntheticGraph:1–2=973.4,
2–3key0=1127.6,3–4=946.8,4–5=1083.2,5–1=864.7,3–6=217.3,2–7=326.9,
2–4=548.6,2–3key1=1189.5. Loop key0=4995.7m; topology/tags/IDs unchanged.
Test1→2 now expects reward9.734/Q3.7301 independently, finite absolute1e-9 checks
and actual stored Q. Logs use4decimals for reward/Q; raw floats are compared.
Active full/single-step PlantUML sources in TA assets synchronized, not rendered.
All tests/vet/build passed. Production/defaults/YAML/notebooks/results/Word stay.

## Verification split: reward first, Q second — 14 November 2026

User explicitly wants small independent tests per concept. Replaced the combined
TestVerificationNormalStep with TestVerification01Reward and TestVerification02QValue,
both in verification_test.go. Each builds a fresh shared synthetic graph and
DefaultConfig/ScenarioA environment. Do not share mutable state/results between tests.
01 uses env.Step only, logs State/Road score/Road reward, asserts score1/reward9.734,
zero other components and nonterminal transition over973.4m; no Q-table/RNG.
02 uses production TrainStep with assigned Q2/maxlegal4 (other legal key Q1),
asserts actual reward9.734/inputQ/transition/Q3.7301 and persisted value; logs
State/Q before/Q value only. Epsilon0/action fixed; RNG required but not sampled.
Prefix01/02 is discussion order, not execution dependency. No Q manual/Q program.
Full tests/vet/build passed; helper/production/defaults/YAML/notebooks/results/
PlantUML/Word unchanged. Old single-test command is obsolete; README lists
individual commands and -run '^TestVerification' for both. No additional cases.

## Two sequential verification episodes — 14 November 2026

User approved replacing the two independent one-step tests with a full two-episode
flow. They explicitly require supplied actions, no epsilon-greedy selection.
TestVerificationEpisodes in verification_test.go now owns one initially empty Q,
resets environment only, runs1→2→3→6(no_available_action,2318.3m,-176.00435),
then1→2→3→4→5→1(success4995.7m,124.011725). All keys0/ScenarioA/DefaultConfig.
TrainStep uses explicit non-nil action; RNG only required by API, never sampled.
Independent expected reward/Qbefore/maxnext/Qafter values asserted each step;
terminal maxnext0, stop/error penalties and preserved old Q3→6 also checked.
Episode1 all Qbefore0; Q1→2=1.4601 then2.9422095, Q2→3=1.6914 then3.12909.

Test-only radial map:1=0,2=973.4,3=1600,4=1400,5=864.7,6=1750,7=1100m.
No geographic coordinates/Haversine/geometry overlap claims. Logs5core lines
per step with8decimal numeric output; one Reward total only for extra nonzero
components, plus episode summary and Q snapshot. No Q manual/Q program labels.
writeVerificationQTable saves deterministic full-precision ScenarioA CSV after
EACH episode (3/6stored pairs, absent pairs0). Snapshots are overwritten in ignored playground/verification/ under the Go
repository root. The writer remains test-local; no dependency on ignored Go code.
No output-directory environment variable is required. No shared
state/dependency between separate Go tests; sequencing is inside this ONE test.

Evidence in TA docs/assets/bab_4_go/verifikasi/dua_episode_20261114 contains two
CSV snapshots and actual verification.log. CSV contents matched independent
expectations. Full go test ./...(8packages),vet,build passed. Production code,
shared graph helper, defaults/YAML/notebooks/results/catalog and Word unchanged.
Only2synthetic episodes, no research training/download/commit/push. Old 01/02
commands obsolete. README explains run and persistent CSV output; actual raw
log file is not automatically overwritten by running the Go test alone.

## Verification A/B/C and action-mask evidence — 14 November 2026

Current TestVerificationEpisodes now runs subtests A/B/C with TWO episodes EACH.
Q starts empty independently per scenario; reset environment only between its
first failed and second successful episode. Explicit actions/key0 bypass policy
sampling only; production TrainStep/mask/reward/Bellman unchanged.
A:1-2-3-6/no_available_action/2318.3m,total-176.00435, then loop4995.7m.
B:1-2-3-4-5-4-3/over_distance/6161m,total-90.5392, then same successful loop.
C:1-2-3-4-3-2/step_limit/5122.2m,total-29.1816, then same successful loop.
Success path1-2-3-4-5-1,total124.011725 for all. C uses test-only max_steps5;
training/default/A/B remain500. Successful C step5 wins over step limit.

Per-step mask logs actual legal actions, compares against independent expected
lists, and explains blocked raw edges. Exercises way/node private, premature
closing, pre-threshold revisit, later permitted revisits and parallel keys;
no claim all visit/access boundaries covered. Five core logs retained with one
state line including cumulative distance, bin(B/C), previous(C). Non-road reward
terms shown only when nonzero. No Q manual/Q program labels or policy tests.
Full expected states/bin/previous, per-step distance/reward/Q inputs/output,
terminal bootstrap0, failure retention, final penalties and Q counts asserted.

Scenario CSV folders now playground/verification/A,B,C (Git-ignored), two files
each; stored counts A3/6,B6/7,C5/7. Header adds scenario/progress_bin/previous_node;
inactive state fields blank, values full precision. Old root CSV/TA evidence are
historical; never confuse them with current subfolder output. Same seven-node
graph and synthetic radial map; no Haversine/geometry/overlap claim. Production,
shared graph helper, YAML/notebooks/research outputs/Word/PlantUML unchanged.
No research training/download/commit/push. README has subtest commands.

Validation: all6episodes/29steps passed;6CSV snapshots(34stored rows across
snapshots) matched independent Decimal expectations within1e-10. Full go test
./...(8packages),go vet,go build and both git diff --check passed.127protected
tracked/shared files unchanged by this task. Actual log captures saved locally
at playground/verification/{A,B,C}/verification.log and combined verification.log;
normal Go command prints logs only and does not refresh those capture files.

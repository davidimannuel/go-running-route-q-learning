# Repository Guidelines

## Collaboration

Proyek ini menulis ulang core TA rute lari Dukuh Kupang dari Python ke Go. Chat berbahasa Indonesia; identifier, komentar kode, dan error berbahasa Inggris. Pengguna belajar lewat coding dan review per komponen. Diskusikan perubahan desain sebelum implementasi.

**Always keep it simple:** utamakan kode idiomatic, mudah dibaca, hemat memori, dan standard library. Hindari abstraksi atau penanganan format yang belum dibutuhkan. Pertahankan referensi dan komentar singkat. Sinkronkan command di README dan `--help` saat CLI berubah; jangan membuat docs tambahan tanpa permintaan. Jaga panduan ringkas.

## Layout

- `cmd/cli`: download, analyze, train, serve, dan penyimpanan hasil.
- `internal/osm`: Overpass, cache, graf mentah, parsing, Haversine.
- `internal/qlearning`: scoring, state, mask, environment, reward, training, metrik.
- `internal/analysis`: ekspor CSV/Q-table dan HTML peta.
- `analysis/`: modul Python untuk analisis graf/training. Notebook hanya parameter, pemanggilan, penjelasan, dan hasil; jangan menduplikasi core Go.
- `workdirs/`: config, cache, output; tanpa folder input. Output/cache, `.venv`, dan playground diabaikan Git.

## Research Rules

Acuan baca saja: `/Users/david/widya_kartika/uwika-ta-31123022-2026`; baca bagian relevan `rute_lari_core.py`, `IMPLEMENTASI.md`, dan metadata hasil.

Pertahankan loop 5.000 m ±10%, state A/B/C, action `(next_node,key)`, mask, dan reward yang disepakati. Reward/scoring dari `qlearning.reward`, default `reward.go`; catat perubahan pada metadata. Access scalar kosong/yes/permissive diperbolehkan; tanpa split/list. Urutan action numerik menurut tujuan lalu key. Jarak ke start memakai Haversine. Overlap berdasarkan hitungan lintasan geometri ruas, menyatukan arah balik/geometri sama.

Graf utama tetap mentah; simplifikasi tidak menjadi lingkup TA saat ini agar atribut setiap segmen tetap asli. Pemilihan komponen dan truncation belum diterapkan. Hasil Go belum identik dengan OSMnx/NumPy. Jangan mengubah metadata/arsip lama atau menganggap seed sama memberi urutan acak lintas bahasa yang sama.

## Workdirs and Validation

Dua workdir aktif: `training` (12 profil A/B/C, seed 0–4) dan `testing` (tiga profil A/B/C, lima seed acak bersama, 150k/horizon 50k). Scenario ada pada setiap profil; lingkungan/reward/parameter lain dipakai bersama. Workdir aktif memakai `max_steps=500` dan `max_workers=5`. Setiap seed memiliki Q-table/environment/RNG mandiri. Pertahankan hasil/metadata arsip saat migrasi. Jangan menjalankan training panjang tanpa permintaan.

Output memiliki metadata JSON; training mencatat konfigurasi efektif/provenance per profil dan seed serta report HTML. Server menyediakan navigasi peta/detail; JSON/CSV tidak disajikan langsung. Notebook membandingkan profil dalam scenario dan A/B/C dengan parameter sepadan dari satu workdir. Peta rute memiliki layer graf Comfort score/Access, default tersembunyi; payload dan scoring berbagi builder dengan peta analisis.

Gunakan `gofmt`, `go test ./...`, `go test -race ./...` untuk konkurensi, `go vet ./...`, dan `go build ./...`. Tes memakai fixture lokal; expected hitungan Q harus independen. Setelah perubahan Python, jalankan notebook terkait dari kernel bersih dan periksa grafik; simpan notebook tanpa output. Commit deskriptif; jangan sertakan artefak lokal.

Prioritaskan review metrik dan verifikasi langkah/reward/Q.


## Latest Audit — 2026-11-07

- Hasil terbaru workdir `training`: revisit **-5**, return1.20, progress_bins16, max_steps500/max_workers5; 12 profil/60 evaluasi/6.750.000 episode selesai 7 November pukul16.00 WIB. Metadata faktor lain, graf/start/seeds identik dengan baseline revisit-10. Snapshot `f0a5885d…`, start `9620836306`.
- Greedy baseline-10→-5: **14/60→28/60**; A0→0/20, B10→17/20, C4→11/20. Loop per konfigurasi B/C (lima seed): dinamis75K=2/0, dinamis150K=5/3, horizon50K-total75K=5/3, horizon50K-total150K=5/5. A seluruh profil0/5.
- Horizon50K/total150K: training10K terakhir B51,196%→67,534%, C25,392%→56,814%. Greedy terbaru B/C: galat229,50/353,60m, comfort0,9248/0,9405, overlap2,88%/4,28%, Q-state59.593/95.582. B dinamis150K juga5/5 dengan galat135,54m, tetapi overlap4,59% dan training-tail38,956%; jangan menyebut horizon unggul seluruh metrik.
- Kegagalan terbaru32:21 over_distance/11 no_available_action; semuanya tidak pernah memiliki closing legal. Tidak ada step_limit pada6,75 juta episode; maksimum353 langkah. Loop valid4.544,87–5.469,86m. Overlap seluruh evaluasi B3,41%→3,90%, C1,45%→3,52%; jumlah kegagalan berubah, jangan mengisolasi mutu rute dari rerata ini.
- Audit latest: seluruh history diperiksa jumlah/urutan episode, seed, jadwal epsilon; metadata hanya berbeda revisit-10→-5. Reward/jarak/overlap8584 langkah konsisten. Seluruh60 greedy direproduksi dari Q-table; evaluation.csv dan evaluation-steps.csv cocok byte-for-byte. Tidak training/unduh atau mengubah core/config saat audit.
- Audit aturan sebelumnya (revisit-10):114 kasus sintetis A/B/C, skala pulang0.60/1.20, cocok Go/Python untuk state/mask/reward/terminasi/bootstrap/Q; replay60 rute/8.989 langkah dan scoring58.020 edge cocok setelah jarak/parameter disamakan di memori. Ini bukan kesetaraan seluruh training/RNG/graf atau jaminan bebas bug. C membagi pengalaman menurut previous_node, tetapi tetap tidak menyimpan seluruh riwayat kunjungan yang memengaruhi mask.
- Keputusan: fokus reward/aturan perjalanan; gamma0.95, alpha, epsilon, state dan rumus Q tetap. Pengurangan revisit-5 disetujui dan sudah dilatih. Pertahankan progress_bins16; eksperimen8/32 belum disetujui. Hasil baik pada start/snapshot/lima seed ini belum membuktikan generalisasi. Total reward lintas penalti tidak dibandingkan langsung sebagai kualitas rute.
- Baseline return1.20/revisit-10 disimpan di `workdirs/training/output/baselines/revisit-10-return-1.20-20261107.json`: metadata/evaluasi/ringkasan training, tanpa history/Q-table lengkap. Baseline lebih lama return0.60 hanya tersisa angka audit. Jangan menimpa baseline.
- Pembanding Python aktif adalah `reward_missing_05_20261103` (CSV/metadata12 konfigurasi diperiksa):23/60 loop, A2/B9/C12; Go terbaru28/60, A0/B17/C11. B lebih banyak loop di seluruh konfigurasi; C horizon75K turun5→3 dan horizon150K tetap5, tetapi galat276,57→353,60m. Go tidak unggul seluruh metrik. Reward pulang1.20 vs0.60, revisit-5 vs-10, max_steps500 vs220, graf/snapshot/start/RNG berbeda; overlap hitungan segmen juga dipengaruhi simplifikasi. Ini perbandingan deskriptif, bukan efek bahasa Go atau satu faktor.
- Sumber aktif: metadata, evaluation.csv, history.csv, dan seed-*/evaluation-steps.csv di `workdirs/training/output/training/<profile>/`. Untuk sesi paralel, baca ulang file sebelum mengedit, pertahankan perubahan sesi lain, dan pisahkan keputusan dari usulan. Tinjau hasil ini sebelum mengubah faktor lain; konfigurasi belum diubah lagi.

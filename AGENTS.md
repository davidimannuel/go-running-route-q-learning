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

Output memiliki metadata JSON; training mencatat konfigurasi efektif/provenance per profil dan seed serta report HTML. Server menyediakan navigasi peta/detail; JSON/CSV tidak disajikan langsung. Notebook membandingkan profil dalam scenario dan A/B/C dengan parameter sepadan dari satu workdir.

Gunakan `gofmt`, `go test ./...`, `go test -race ./...` untuk konkurensi, `go vet ./...`, dan `go build ./...`. Tes memakai fixture lokal; expected hitungan Q harus independen. Setelah perubahan Python, jalankan notebook terkait dari kernel bersih dan periksa grafik; simpan notebook tanpa output. Commit deskriptif; jangan sertakan artefak lokal.

Prioritaskan review metrik dan verifikasi langkah/reward/Q.


## Latest Audit — 2026-11-07

- Audit workdir `training`: 12 profil, 60 evaluasi, 6.750.000 episode; metadata `max_steps=500`, `max_workers=5`, `gamma=0.95`, epsilon akhir 0.05. Snapshot `f0a5885d…`, start `9620836306`.
- Greedy tetap 5/60 loop: A 0/20, B 4/20, C 1/20. Gagal: 37 `over_distance`, 18 `no_available_action`; seluruh kegagalan tidak pernah memiliki closing legal. Tidak ada `step_limit` selama training/evaluasi; maksimum teramati 364 langkah.
- Dibanding audit 220 langkah, keberhasilan training 10K terakhir pada horizon50K/total150K: B 14,12%→19,58%; C 0,60%→10,85%; A 0,32%→0,032%. Baseline greedy 220 juga 5/60; raw output sebelumnya sudah ditimpa.
- Diagnosis: penjumlahan reward/jarak pada 60 trace konsisten dengan evaluasi. Ada rute B dengan 24 revisit/penalti -240; beberapa kegagalan tanpa revisit tetap bergerak menjauh saat fase pulang.
- Keputusan terbaru pengguna: fokus pada reward dan aturan internal perjalanan. Jangan mengubah gamma (tetap 0.95), alpha, epsilon, representasi state, atau rumus update Q pada tahap ini. Usulan gamma0.975 sebelumnya tidak dilanjutkan.
- Kandidat untuk dibahas, BELUM diterapkan: kekuatan reward arah pulang (`return_reward_scale` 0.60→1.20), lalu penalti revisit (-10→-5) sebagai eksperimen terpisah. Diskusikan perubahan reward/mask/terminasi sebelum implementasi; ubah satu faktor dan simpan baseline500 sebelum rerun.
- Sumber: metadata, `evaluation.csv`, `history.csv`, dan `seed-*/evaluation-steps.csv` di `workdirs/training/output/training/<profile>/`. Untuk sesi paralel, baca ulang file sebelum mengedit dan jangan menimpa perubahan sesi lain. Catat keputusan yang disetujui terpisah dari usulan.

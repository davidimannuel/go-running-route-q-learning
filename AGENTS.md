# Repository Guidelines

## Collaboration

Proyek ini menulis ulang core TA rute lari Dukuh Kupang dari Python ke Go. Gunakan bahasa Indonesia di chat; identifier, komentar kode, dan error berbahasa Inggris. Pengguna belajar lewat coding dan review per komponen. Implementasikan saat diminta; diskusikan perubahan desain dahulu.

**Always keep it simple:** utamakan kode idiomatic, mudah dibaca, hemat memori, dan standard library. Hindari abstraksi atau penanganan format yang belum dibutuhkan. Pertahankan referensi kode dengan komentar singkat yang menjelaskan tujuan. Dokumentasi penggunaan di README; jangan membuat folder/file docs tambahan tanpa permintaan. Jaga panduan ini tetap ringkas.

## Layout

- `cmd/cli`: download, analyze, train, serve, worker, dan penyimpanan hasil.
- `internal/osm`: Overpass, cache, graf mentah, parsing, Haversine.
- `internal/qlearning`: scoring, state, mask, environment, reward, training, metrik.
- `internal/analysis`: ekspor CSV/Q-table dan HTML peta.
- `analysis/`: modul Python untuk analisis graf/training. Notebook hanya parameter, pemanggilan, penjelasan, dan hasil; jangan menduplikasi core Go.
- `workdirs/`: config, cache, output; tanpa folder input. Output/cache, `.venv`, dan playground diabaikan Git.

## Research Rules

Acuan baca saja: `/Users/david/widya_kartika/uwika-ta-31123022-2026`; baca bagian relevan `rute_lari_core.py`, `IMPLEMENTASI.md`, dan metadata hasil.

Pertahankan loop 5.000 m ±10%, state A/B/C, action `(next_node,key)`, mask, dan reward yang disepakati. Konstanta reward ada di `reward.go`; jangan mengubah metode demi mengejar hasil. Access scalar kosong/yes/permissive diperbolehkan; tanpa split/list. Urutan action numerik menurut tujuan lalu key. Jarak ke start memakai Haversine. Overlap berdasarkan hitungan lintasan geometri ruas, menyatukan arah balik/geometri sama.

Graf masih mentah; simplifikasi, pemilihan komponen, dan truncation dibahas di akhir. Hasil Go belum identik dengan OSMnx/NumPy. Jangan mengubah metadata/arsip lama atau menganggap seed sama memberi urutan acak lintas bahasa yang sama.

## Workdirs and Validation

Empat workdir utama: scenario-a/b/c (empat profil, seed 0–4) dan testing (B, lima seed acak, 150k episode, horizon 50k). Detail config dan command di README. `dukuh-kupang` menyimpan snapshot bersama penelitian. Setiap seed memiliki Q-table/environment/RNG mandiri; hasil tertimpa sesuai workdir/profil. Jangan menjalankan training panjang tanpa permintaan.

Gunakan `gofmt`, `go test ./...`, `go test -race ./...` untuk konkurensi, `go vet ./...`, dan `go build ./...`. Tes memakai fixture lokal; expected hitungan Q harus independen. Setelah perubahan Python, jalankan notebook terkait dari kernel bersih dan periksa grafik; simpan notebook tanpa output. Commit deskriptif; jangan sertakan artefak lokal.

Review berikutnya: metrik dan orkestrasi CLI. Prioritaskan verifikasi langkah/reward/Q sebelum improvement graf.

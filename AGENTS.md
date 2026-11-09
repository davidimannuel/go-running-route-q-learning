# Repository Guidelines

## Collaboration

Proyek ini menulis ulang core TA rute lari dari Python ke Go. Chat berbahasa Indonesia; identifier, komentar kode, dan error berbahasa Inggris. Pengguna belajar melalui coding dan review per komponen. Diskusikan perubahan desain sebelum implementasi.

**Always keep it simple:** utamakan kode idiomatic, mudah dibaca, standard library, serta penggunaan memori yang wajar. Hindari abstraksi dan format yang belum dibutuhkan. Pertahankan referensi dan komentar singkat. Sinkronkan README dan bantuan CLI; jangan membuat dokumen tambahan tanpa permintaan.

## Project Layout

- `cmd/cli`: download, analyze, train, serve, dan penyimpanan hasil.
- `internal/osm`: Overpass, cache, graf mentah, Haversine, parsing width.
- `internal/qlearning`: scoring, state, mask, environment, reward, Q-table, metrik.
- `internal/analysis`: CSV, HTML peta, Q-table, dan report metadata.
- `internal/config`, `internal/logger`, `internal/htmlserver`: konfigurasi, log, navigasi hasil.
- `analysis/`: core analisis Python; notebook hanya parameter, pemanggilan, dan penjelasan.
- `workdirs/`: config, cache, output; tanpa folder input. Output/cache/playground diabaikan Git.

## Research Rules

Acuan baca saja: `/Users/david/widya_kartika/uwika-ta-31123022-2026`. Rujukan teknisnya `rute_lari_core.py` dan `IMPLEMENTASI.md`; analisis final memakai hasil proyek Go sendiri.

Pertahankan target 5.000 m ±10% dan kembali ke start. State A=current node, B=ditambah progress bin, C=ditambah previous node. Action `(next_node,key)` diurutkan numerik. Akses scalar kosong/yes/permissive wajib lolos pada way dan kedua endpoint, tanpa split/list atau override foot. Start yang tidak lolos ditolak tanpa resnap; barrier saja tidak melarang. Penalti permissive tetap dari way. Policy baru `edge_and_endpoint_whitelist_v1` berlaku 9 November; hasil 7 November memakai way saja. Hasil terbaru dapat bercampur versi kebijakan; cek metadata tiap profil. Pertahankan baseline; gunakan output terpisah untuk eksperimen baru. Warna Access jalan tetap tag way; overlay Node access default mati pada kedua peta, memakai tag access/barrier node dan status whitelist dari Go. Akses node di peta analisis memakai satu checkbox Node access / barriers di bawah Show nodes; tanpa duplikat kanan bawah. Server/iframe memuat ulang HTML tanpa cache. Legenda menghitung node bertag. Diagram analisa tetap edge; node untuk struktur/validasi CSV.

Graf tetap mentah dan bidirectional; simplifikasi di luar lingkup saat ini. Seleksi komponen/truncation belum diterapkan. Jarak memakai Haversine; hasil tidak identik dengan graf proyeksi/simplifikasi atau RNG Python. Overlap adalah hitungan lintasan ruas fisik berdasarkan endpoint, menggabungkan arah balik/geometri identik. Jalan berbeda yang berdekatan bukan overlap. Skor highway/surface/width adalah preferensi penelitian, bukan keamanan aktual.

## Final Experiment

Workdir `training`: A/B/C × dinamis150k/300k dan horizon100k-total150k/300k, 12 profil/60 training/13.500.000 episode. Seed tetap0–4, alpha0.15, gamma0.95, epsilon1→0.05, evaluasi greedy0. Setiap profil/seed mulai dari Q-table/RNG mandiri. `testing`: 12 profil dan parameter yang sama, dengan koordinat sendiri dan lima seed acak bersama per command; total13.500.000episode bila semua profil dijalankan. Nama profil kedua workdir identik; jangan memakai alias lama a/b/c.

Parameter bersama: 25 bin, max_steps500, max_workers5, outward0.60, return1.20, revisit−5. Reward lain ada di YAML. Jangan menambah variasi atau menjalankan training panjang tanpa permintaan. Rancangan dikunci; prioritas berikutnya analisis BAB IV dan penyesuaian Bab I–III.

Pengguna menyelesaikan hasil final 7 November 2026 pukul18.25 WIB: greedy31/60, A1/20, B18/20, C12/20. Urutan dinamis150k/300k dan horizon-total150k/300k: A=0/1/0/0, B=5/5/3/5, C=0/3/4/5, masing-masing dari5. Angka ini checkpoint historis 7 November; output terbaru dapat sudah dilatih ulang. Tabel ringkasan ada di README; sumber angka terkini tetap CSV/metadata tiap profil.

Audit: seluruh13,5juta history diperiksa;60 evaluasi dan8.640 baris trace direproduksi dari Q-table byte-for-byte. Semua29 gagal tanpa closing legal:25 over_distance/4 buntu; tidak ada step_limit, maksimum353 langkah. Ringkasan lokal: `workdirs/training/output/analysis/final-experiment-audit.json`. Output pendahuluan/arsip lama dihapus atas permintaan pengguna; hasil final sekarang dipertahankan.

## Maps and Validation

Peta mempertahankan seed, metrik, marker input/start/finish/KM, serta layer Comfort/Access. Klik node biasa/akses membuka outgoing neighbors (ID/key/panjang/access) dan sorotan koneksi; semua edge mentah termasuk yang ditolak akses, bukan mask episode. Show graph nodes pada rute memakai zoom>=17/viewport/maks2.000 marker; popup jalan/START juga menyediakan inspeksi. Tutup popup membersihkan sorotan. Route distance memilih seluruh rute/per-KM; konteks lain samar. Sorotan ruas ulang dan panah arah default mati. Popup memilih setiap traversal beserta step/interval/reward. Pembaruan 9 November menambahkan tag/status node pada payload peta tersimpan; rute, metrik, skor edge, dan artefak training tetap. Jangan menganggap rute baseline dilatih ulang dengan mask node. Desktop/mobile diuji.

Jangan menyimpulkan optimalitas, generalisasi, atau validasi lapangan dari lima seed. Laporkan kegagalan dan jumlah sampel saat memakai rerata rute valid. Overlap rendah belum menjamin bentuk rute yang memuaskan. Jangan memakai angka arsip sebagai hasil baru atau mengubah metadata lama.

Gunakan gofmt, `go test ./...`, `go vet ./...`, `go build ./...`; tambah `go test -race ./...` untuk perubahan konkurensi. Tes memakai fixture lokal dan expected Q/reward independen. Setelah perubahan Python, jalankan notebook terkait dari kernel bersih dan periksa grafik. Commit deskriptif; simpan notebook tanpa output dan jangan sertakan artefak lokal. Baca ulang file sebelum edit untuk menjaga perubahan sesi lain.

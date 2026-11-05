# Repository Guidelines

## Tujuan dan Cara Berkolaborasi

Repository ini menulis ulang core TA rekomendasi rute lari Dukuh Kupang, Surabaya, dari Python ke Go. Pengguna menulis kode Go; agent membantu perencanaan, penjelasan, review idiomatic Go, pengujian, analisis, dan pembuatan HTML peta. Jangan mengambil alih implementasi tanpa permintaan pengguna. Gunakan bahasa Indonesia dan kerjakan satu komponen kecil sampai dipahami sebelum integrasi.

## Project Structure & Module Organization

- `go.mod`: modul `github.com/davidimannuel/go-running-route-q-learning`; versi Go mengikuti berkas ini.
- `cmd/`: tempat entry point CLI; saat inisialisasi masih kosong.
- `internal/`: tempat implementasi internal; saat inisialisasi masih kosong. Tambahkan package sesuai kebutuhan komponen.
- Belum ada pengujian, notebook, atau aset dalam repository ini. Letakkan pengujian bersama package terkait dan fixture kecil dalam `testdata/` ketika diperlukan.

Core dan training berjalan di Go. Jupyter hanya membaca hasil untuk analisis dan diagram. Utamakan standard library; jelaskan kebutuhan dependensi eksternal sebelum menambahkannya.

## Referensi dan Kesetaraan Perilaku

Proyek acuan berada di `/Users/david/widya_kartika/uwika-ta-31123022-2026`. Baca `rute_lari_core.py`, bagian relevan `IMPLEMENTASI.md`, notebook, serta metadata eksperimen. Catatan historis dapat berbeda dari konfigurasi aktif. Perlakukan proyek acuan sebagai referensi baca saja.

Pertahankan target loop 5.000 m dengan toleransi 10%, action edge paralel, state A/B/C, action mask, reward, dan metrik sesuai baseline yang disepakati. Lima seed penelitian adalah `0,1,2,3,4`. Catat perubahan desain; jangan menganggap seed sama menjamin urutan acak identik antara Python dan Go. Bahas simplifikasi graf, proyeksi koordinat, dan format artefak sebelum menetapkannya.

## Build, Test, and Development Commands

Setelah package tersedia, jalankan dari root:

- `gofmt -w <berkas.go>`: format berkas Go yang diubah.
- `go build ./...`: kompilasi seluruh package.
- `go test ./...`: jalankan pengujian.
- `go vet ./...`: periksa masalah statis umum.

Belum ada perintah CLI yang dapat dijalankan.

## Coding Style & Naming Conventions

Ikuti `gofmt`, nama package pendek lowercase, nama exported `MixedCaps`, dan nama internal `mixedCaps`. Sertakan satuan seperti `DistanceM`. Tangani error dengan konteks; gunakan `context.Context` dan timeout untuk HTTP. Komentar berbahasa Indonesia menjelaskan alasan aturan penelitian.

## Testing Guidelines

Gunakan `testing`, table-driven tests, dan `httptest` untuk Overpass. Uji fixture lokal, edge paralel, batas jarak, mask, reward, serta update Q dengan hitungan independen. Nama berkas `*_test.go`, fungsi `TestXxx`. Belum ada target coverage numerik. Pisahkan tes dari unduhan langsung dan training panjang.

## Commit & Pull Request Guidelines

Repository Go belum memiliki riwayat Git. Acuan Python menggunakan pesan deskriptif tanpa konvensi prefix wajib. Buat commit per komponen. PR menjelaskan tujuan, perubahan perilaku, hasil pemeriksaan, dan dampak terhadap kesetaraan eksperimen; sertakan contoh CSV atau tampilan peta bila relevan.

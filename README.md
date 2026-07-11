# Keuangan Sekolah

Aplikasi pencatatan keuangan sekolah berbasis web (Go + Gin + GORM + MySQL, server-side HTML dengan HTMX & Tailwind).

## Status
**Phase 1 – Fondasi** ✅
Autentikasi JWT, middleware role, koneksi MySQL, layout dasar (sidebar + topbar), login, dashboard.

**Phase 2 – Master Data** ✅
CRUD Tahun Ajaran (set aktif), Kelas, Siswa, Jenis Pemasukan, Kategori + Sub Kategori.
Pola UI: modal HTMX, validasi inline, tabel ter-update tanpa reload (OOB swap), toast, proteksi hapus referensial. Admin-only.

**Phase 6 – Tutup Tahun & Piutang** ✅
Tutup Tahun (admin): ringkasan akhir tahun, **kunci TA lama** (transaksi kas dibekukan), **buka & aktifkan TA baru**, **bawa saldo kas** (jenis "Saldo Awal"), dan ubah **sisa tunggakan (SPP+Tagihan) jadi Piutang** per siswa. **Batal Tutup Tahun** (selama TA baru masih bersih). Menu **Piutang**: bayar (cicil) — pembayaran tercatat sebagai kas di **TA aktif saat pembayaran** (jenis "Piutang"), riwayat & hapus. Peringatan bila tabungan belum ditutup.

**Phase 4 – SPP** ✅
Generate tagihan bulk (per kelas/semua kelas, pilih bulan, nominal/bulan, anti-dobel), pembayaran per siswa (prefilled sisa = lunas, dukung cicil), status otomatis (belum/cicil/lunas), rekap tunggakan per kelas. Tulis: admin & bendahara; kepala sekolah lihat.
Tiap pembayaran SPP **otomatis tercatat sebagai Kas Pemasukan jenis "SPP"** (auto-posting); baris kas itu terkunci (tak bisa dihapus manual), jenis "SPP" ditandai bawaan sistem dan tidak muncul di input kas manual.
Tiap baris siswa punya tombol **Riwayat** → lihat semua pembayaran & **hapus** pembayaran salah input; menghapus pembayaran otomatis menghapus baris kas tertaut dan menghitung ulang status (lunas/cicil/belum).

**Tagihan (non-SPP)** ✅
Pembayaran insidental non-bulanan (Uang Seragam, Buku, Daftar Ulang, dll). Master **Jenis Tagihan**. Menu **Tagihan**: terbitkan ke target fleksibel (per kelas / seluruh sekolah / pilih siswa) dengan **nominal bisa beda per siswa** (isi cepat "terapkan ke semua"), bayar per siswa (**cicil**, status belum/cicil/lunas), rekap **tunggakan** per kelas, riwayat & hapus. Pembayaran **auto masuk Kas** (jenis "Tagihan", terkunci & sinkron saat hapus). Hapus tagihan menarik balik pembayaran + baris kas.

**Dana Bantuan / Donatur** ✅
Master **Donatur** + tab **SPP → Dana Bantuan**. Catat dana dari donatur (mis. "Mentari"), pilih siswa & bulan, sistem alokasikan ke SPP otomatis: alokasi per slot = `min(nominal donatur, sisa tagihan)`. Kelebihan → kas "Donasi"; kekurangan → SPP jadi cicil (ditutup tunai/donatur lain). Satu SPP boleh ditambal banyak donatur. Tidak ada penghitungan kas ganda (lump-sum tidak diposting; terurai jadi baris SPP + donasi). Ada rekap per donatur, detail alokasi, dan hapus (menarik balik pembayaran + baris kas + menghitung ulang status).

**Phase 5 – Tabungan Siswa** ✅
Setoran per siswa & **bulk per kelas**, saldo & riwayat per siswa (riwayat bisa **difilter per bulan**), hapus setoran. Persen potongan di tabel `setting` (ubah kapan saja; hanya berlaku untuk setoran baru — persen di-**snapshot** per setoran).
**Potongan belum masuk kas selama tabungan belum ditutup**: selama tahun berjalan saldo siswa = setoran penuh, dan potongan hanya ditampilkan sebagai *perkiraan saat tutup*. Potongan baru diakui & masuk Kas ("Potongan Tabungan") saat **Tutup Tabungan** — event seluruh sekolah (per tahun ajaran), terpisah dari Tutup Tahun. Saat tutup, per siswa: potongan → kas; **saldo bersih otomatis melunasi tunggakan** (SPP dulu per bulan, lalu Tagihan per tanggal) → jadi pemasukan kas; **sisa** diserahkan ke siswa (tidak memengaruhi kas). Setelah tutup, setoran dikunci; **Batal Tutup** me-reverse semuanya (potongan + pelunasan + status). Form setor punya pratinjau real-time.

**Phase 3 – Transaksi Kas** ✅
Kas Pemasukan & Pengeluaran (input rupiah berformat otomatis, pilih jenis/kategori dari master, sub-kategori dependen), filter tahun ajaran + bulan, saldo kas real-time, dashboard ringkasan kas. Tulis: admin & bendahara; kepala sekolah lihat saja. Nilai uang disimpan int64 rupiah (akurat, tanpa galat float).

## Menjalankan

1. Pastikan MySQL berjalan, lalu sesuaikan `.env` (lihat `.env.example`).
   Database dibuat otomatis bila belum ada.
2. Install dependency & jalankan:

   ```powershell
   go mod tidy
   go run .
   ```

3. Buka http://localhost:8080
   Login dengan akun admin awal dari `.env`
   (default: `admin@sekolah.test` / `admin123`) — **ganti setelah login**.

### Live reload (opsional)
```powershell
go install github.com/air-verse/air@latest
air
```

## Struktur
```
config/      koneksi DB & konfigurasi env
models/      definisi tabel (GORM) + migrasi
auth/        JWT & hashing password
middleware/  auth (cek cookie JWT) & RBAC (cek role)
handlers/    controller per modul
routes/      pendaftaran rute
templates/   HTML (partials, auth, dashboard)
static/      css, js, img
```

## Roadmap
Lihat `PLAN_Keuangan_Sekolah.md` — Phase 2 (Master Data) berikutnya.
